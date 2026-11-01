/**
 * @jest-environment jsdom
 */
/**
 * 能力検査の受検画面（SOCAIAGENT-433）。
 *
 * 時間切れの扱いが肝。サーバが締め切りを決めるので、クライアントは
 *   - 残り時間を表示する
 *   - 0 になったら回答させない
 *   - 410 が返ったら「保存に失敗」ではなく「時間切れ」と伝える
 *   - 時間切れでも提出はできる（弾くと受検した意味が無くなる）
 * を満たす必要がある。
 */
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import PageContent from '@/app/ability-assessments/[sessionId]/page-content'
import { AbilityApiError, abilityApi, type AbilitySessionView } from '@/lib/ability'

const push = jest.fn()
const replace = jest.fn()
// router は毎回同じ参照を返す。新しいオブジェクトを返すと useCallback([router]) が
// 作り直され、useEffect が再実行されて取得が止まらなくなる（本番の useRouter は安定）。
const router = { push, replace }
jest.mock('next/navigation', () => ({
  useParams: () => ({ sessionId: 'sess-1' }),
  useRouter: () => router,
}))
jest.mock('@/lib/ability', () => ({
  ...jest.requireActual('@/lib/ability'),
  abilityApi: jest.fn(),
}))

const mockApi = abilityApi as jest.MockedFunction<typeof abilityApi>

const view = (over: Partial<AbilitySessionView> = {}): AbilitySessionView => ({
  session_id: 'sess-1',
  title: '能力検査（言語・非言語）',
  status: 'in_progress',
  questions: [
    { id: 1, position: 1, section: 'verbal', body: '「審議」の意味は', choices: ['調べて話し合う', '通知する', '仰ぐ', '前例に従う'] },
    { id: 2, position: 2, section: 'numerical', body: '原価800円に25%の利益', choices: ['825円', '1000円', '1025円', '1200円'] },
  ],
  answers: {},
  remaining_seconds: 900,
  ...over,
})

/** GET だけ view を返し、PUT/POST は成功扱いにする既定のモック。 */
function mockSession(v: AbilitySessionView) {
  mockApi.mockImplementation(async (path, init) => {
    if (!init) return v as never
    return undefined as never
  })
}

describe('能力検査の受検画面', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    jest.useRealTimers()
  })

  it('問題と選択肢を出し、正解は画面に出さない', async () => {
    mockSession(view())
    await act(async () => { render(<PageContent />) })

    expect(await screen.findByText(/「審議」の意味は/)).toBeInTheDocument()
    expect(screen.getByLabelText('調べて話し合う')).toBeInTheDocument()
    // 正解の印が付いていないこと（どの選択肢も見た目で区別されない）
    expect(document.body.textContent).not.toMatch(/正解|correct/i)
  })

  it('残り時間を表示する', async () => {
    mockSession(view({ remaining_seconds: 900 }))
    await act(async () => { render(<PageContent />) })
    expect(await screen.findByTestId('ability-remaining')).toHaveTextContent('15:00')
  })

  it('残り1分以下は警告を添える', async () => {
    mockSession(view({ remaining_seconds: 45 }))
    await act(async () => { render(<PageContent />) })
    const el = await screen.findByTestId('ability-remaining')
    expect(el).toHaveTextContent('0:45')
    expect(el).toHaveTextContent('まもなく締め切られます')
  })

  it('残り0秒になったら回答させず、提出だけを促す', async () => {
    jest.useFakeTimers()
    mockSession(view({ remaining_seconds: 2 }))
    render(<PageContent />)
    await waitFor(() => expect(screen.getByText(/「審議」の意味は/)).toBeInTheDocument())

    await act(async () => { jest.advanceTimersByTime(3000) })

    expect(screen.getByTestId('ability-time-up')).toBeInTheDocument()
    expect(screen.getByLabelText('調べて話し合う')).toBeDisabled()
    expect(screen.getByRole('button', { name: '提出する' })).toBeEnabled()
  })

  it('回答の保存が410なら「保存できませんでした」ではなく時間切れとして伝える', async () => {
    // 制限時間なしの状態から始める。残り時間のタイマーを動かさず、410 の扱いだけを見る。
    mockApi.mockImplementation(async (path, init) => {
      if (!init) return view({ remaining_seconds: undefined }) as never
      throw new AbilityApiError(410, '制限時間を過ぎました')
    })
    await act(async () => { render(<PageContent />) })
    const choice = await screen.findByLabelText('調べて話し合う')
    await act(async () => { fireEvent.click(choice) })

    expect(screen.getByTestId('ability-time-up')).toBeInTheDocument()
    expect(screen.getByText(/制限時間を過ぎたため、この回答は記録されていません/)).toBeInTheDocument()
    expect(screen.queryByText(/もう一度選んでください/)).not.toBeInTheDocument()
  })

  it('読み込みが410なら、提出だけできる画面にする', async () => {
    mockApi.mockImplementation(async (path, init) => {
      if (!init) throw new AbilityApiError(410, '制限時間を過ぎました')
      return undefined as never
    })
    await act(async () => { render(<PageContent />) })

    expect(await screen.findByTestId('ability-time-up')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '提出する' })).toBeEnabled()
    expect(screen.queryByText(/「審議」の意味は/)).not.toBeInTheDocument()
  })

  it('未回答でも提出できる（飛ばした問は誤答として数える）', async () => {
    mockSession(view({ questions: [view().questions[0]] }))
    await act(async () => { render(<PageContent />) })
    const submit = await screen.findByRole('button', { name: '提出する' })
    expect(submit).toBeEnabled()

    await act(async () => { fireEvent.click(submit) })
    expect(mockApi).toHaveBeenCalledWith('/sessions/sess-1/submit', { method: 'POST' })
  })

  it('提出済みなら結果画面へ送る', async () => {
    mockSession(view({ status: 'submitted' }))
    await act(async () => { render(<PageContent />) })
    await waitFor(() => expect(replace).toHaveBeenCalledWith('/ability-assessments/sess-1/result'))
  })

  it('提出中は選び直させない（提出のPOSTが新しいPUTを追い越すのを防ぐ）', async () => {
    let releaseSubmit: () => void = () => {}
    mockApi.mockImplementation(async (path, init) => {
      if (!init) return view() as never
      if (path.endsWith('/submit')) {
        await new Promise<void>((r) => { releaseSubmit = r })
        return undefined as never
      }
      return undefined as never
    })
    await act(async () => { render(<PageContent />) })
    const choice = await screen.findByLabelText('調べて話し合う')
    await act(async () => { fireEvent.click(choice) })

    // 1問目で提出まで進める
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: '次へ' })) })
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: '提出する' })) })

    // 提出が終わるまで選択肢は無効
    expect(screen.getByLabelText('825円')).toBeDisabled()
    await act(async () => { releaseSubmit() })
  })

  it('中断は保存の結果を待ち、失敗があれば遷移しない', async () => {
    mockApi.mockImplementation(async (path, init) => {
      if (!init) return view() as never
      if (path.includes('/answers/')) throw new AbilityApiError(500, 'boom')
      return undefined as never
    })
    await act(async () => { render(<PageContent />) })
    const choice = await screen.findByLabelText('調べて話し合う')
    await act(async () => { fireEvent.click(choice) })

    await act(async () => { fireEvent.click(screen.getByRole('button', { name: '中断して一覧へ戻る' })) })

    expect(push).not.toHaveBeenCalledWith('/ability-assessments')
    expect(screen.getByText(/保存できていない回答があります/)).toBeInTheDocument()
  })

  it('中断のラベルで「保存済み」と断定しない', async () => {
    mockSession(view())
    await act(async () => { render(<PageContent />) })
    const leave = await screen.findByRole('button', { name: '中断して一覧へ戻る' })
    expect(leave.textContent).not.toContain('保存済み')
  })

  it('保存が失敗したら、同じ回答を再送できる', async () => {
    let fail = true
    mockApi.mockImplementation(async (path, init) => {
      if (!init) return view() as never
      if (path.includes('/answers/') && fail) throw new AbilityApiError(500, 'boom')
      return undefined as never
    })
    await act(async () => { render(<PageContent />) })
    const choice = await screen.findByLabelText('調べて話し合う')
    await act(async () => { fireEvent.click(choice) })
    expect(screen.getByText(/保存できませんでした/)).toBeInTheDocument()

    // 同じ選択肢を選び直しても RadioGroup の onChange は出ないので、明示的な再送を置く
    fail = false
    const retry = screen.getByRole('button', { name: 'この回答をもう一度保存する' })
    await act(async () => { fireEvent.click(retry) })
    expect(screen.getByText('✓ 保存しました')).toBeInTheDocument()
  })

  it('選んだ添字をそのまま送る（0 も送る）', async () => {
    mockSession(view())
    await act(async () => { render(<PageContent />) })
    const choice = await screen.findByLabelText('調べて話し合う')
    await act(async () => { fireEvent.click(choice) })

    expect(mockApi).toHaveBeenCalledWith('/sessions/sess-1/answers/1', { method: 'PUT', body: { choice: 0 } })
  })
})
