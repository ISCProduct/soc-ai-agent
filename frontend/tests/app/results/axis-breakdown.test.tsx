/**
 * @jest-environment jsdom
 */
import { act, render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import ResultsListView from '@/app/results/components/ResultsListView'
import AxisBreakdownPanel from '@/app/results/components/AxisBreakdownPanel'
import MyCategoryScoresCard from '@/app/results/components/MyCategoryScoresCard'
import { toMyCategoryScores, type AxisBreakdown } from '@/lib/axis-match'
import type { Company } from '@/app/results/types'

// SOCAIAGENT-611: 10カテゴリのスコアと、企業要件との軸別のマッチ。

jest.mock('@/lib/auth', () => ({
  authService: { getUserFetchHeaders: () => ({ 'X-User-Token': 'jwt' }) },
}))

const BREAKDOWN: AxisBreakdown = {
  match_id: 10,
  company_id: 1,
  company_name: 'サンプル株式会社',
  axes: [
    { key: 'technical', label: '技術力', user_score: 72, company_weight: 80, match: 92 },
    { key: 'growth', label: '成長意欲', user_score: null, company_weight: 70, match: null },
    { key: 'detail', label: '緻密さ', user_score: 90, company_weight: 0, match: 10 },
  ],
  strengths: [{ key: 'technical', text: 'あなたの「技術力」（72）は、この企業が重視している度合い（80）と近く、一致しています。' }],
  gaps: [{ key: 'stability', text: 'この企業は「安定志向」を重視しています（80）。あなたの今のスコアは 40 です。関係する経験を具体的に話せるようにすると、伝わりやすくなります。' }],
  unmeasured: ['成長意欲'],
}

function respond(routes: (url: string) => [number, unknown]) {
  const urls: string[] = []
  global.fetch = jest.fn(async (url: RequestInfo | URL) => {
    urls.push(String(url))
    const [status, body] = routes(String(url))
    return { ok: status < 400, status, json: async () => body }
  }) as unknown as typeof fetch
  return urls
}

test('10軸のスコアをキーで固定の並びにそろえ、無い軸・数値でない値は未計測（null）にする', () => {
  const got = toMyCategoryScores([
    { key: 'stability', label: '安定志向', score: 41.6 },
    // 照合はキーで行う。Backend の表示名が変わっても未計測にならない
    { key: 'technical', label: '別の表示名', score: 80 },
    { key: 'teamwork', label: 'チームワーク', score: null },
  ])
  expect(got).toHaveLength(10)
  // 表示名は推薦の一覧の「噛み合った軸」と同じ辞書（CATEGORY_SCORE_LABELS）
  expect(got[0]).toEqual({ key: 'technical', label: '技術力', score: 80 })
  expect(got[4]).toEqual({ key: 'stability', label: '安定志向', score: 42 })
  expect(got[1]).toEqual({ key: 'teamwork', label: 'チームワーク', score: null })
})

describe('あなたの10の傾向', () => {
  test('10軸を固定の並びで、数値も文字で出す。測れていない軸は「未計測」', async () => {
    const urls = respond(() => [200, { axes: [{ key: 'technical', label: '技術力', score: 80 }] }])
    render(<MyCategoryScoresCard sessionId="s 1" />)
    const tech = (await screen.findByText('技術力')).closest('li') as HTMLElement
    expect(within(tech).getByText('80')).toBeInTheDocument()
    expect(within(screen.getByText('チームワーク').closest('li') as HTMLElement).getByText('未計測')).toBeInTheDocument()
    expect(screen.getAllByText('未計測')).toHaveLength(9)
    // 棒は読み上げない（数値と二重に読まれないように）
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    // 軸ごとの内訳と同じ、適性検査を合流させた値を取る（合流前の /api/chat/scores ではない）
    expect(urls[0]).toBe('/api/chat/axis-scores?session_id=s%201')
  })

  test('スコアがまだ無ければ、対話を進めるよう案内する', async () => {
    respond(() => [200, { axes: [] }])
    render(<MyCategoryScoresCard sessionId="s1" />)
    expect(await screen.findByText(/まだスコアがありません/)).toBeInTheDocument()
  })

  test('読み込みに失敗したら知らせ、再読み込みできる', async () => {
    let ok = false
    respond(() => (ok ? [200, { axes: [{ key: 'technical', label: '技術力', score: 80 }] }] : [500, {}]))
    render(<MyCategoryScoresCard sessionId="s1" />)
    expect(await screen.findByText('傾向のスコアを読み込めませんでした。')).toBeInTheDocument()
    ok = true
    fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
    expect(within((await screen.findByText('技術力')).closest('li') as HTMLElement).getByText('80')).toBeInTheDocument()
  })
})

describe('軸ごとの内訳', () => {
  test('軸ごとの数値と、合っている点・補う点・未計測の軸を出す', async () => {
    const urls = respond(() => [200, BREAKDOWN])
    render(<AxisBreakdownPanel matchId={10} />)
    const table = await screen.findByRole('table')
    const tech = within(table).getByRole('row', { name: /技術力/ })
    expect(within(tech).getByText('72')).toBeInTheDocument()
    expect(within(tech).getByText('80')).toBeInTheDocument()
    expect(within(tech).getByText('92')).toBeInTheDocument()
    expect(within(within(table).getByRole('row', { name: /成長意欲/ })).getByText('未計測')).toBeInTheDocument()
    // 企業の重視度が 0 の軸は一致度を出さない
    const detail = within(table).getByRole('row', { name: /緻密さ/ })
    expect(within(detail).getByText('—')).toBeInTheDocument()
    expect(within(detail).queryByText('10')).not.toBeInTheDocument()
    expect(screen.getByText(/重視度が 0 の軸は、一致度を「—」/)).toBeInTheDocument()

    expect(screen.getByRole('heading', { level: 3, name: '合っている点' })).toBeInTheDocument()
    expect(screen.getByText(/「技術力」（72）は/)).toBeInTheDocument()
    expect(screen.getByText('補うと伝わりやすくなる点')).toBeInTheDocument()
    expect(screen.getByText(/「成長意欲」は、まだ診断で測れていません/)).toBeInTheDocument()
    expect(screen.getByText(/AI との対話（と適性検査）から推定した参考値/)).toBeInTheDocument()
    expect(screen.getByText(/推薦の順位を決めるものではありません/)).toBeInTheDocument()
    expect(urls[0]).toBe('/api/chat/matches/10/axes')
  })

  test('見られない推薦（404）と、読み込みの失敗を区別して知らせる', async () => {
    respond(() => [404, {}])
    const { unmount } = render(<AxisBreakdownPanel matchId={10} />)
    expect(await screen.findByText('この企業の内訳は見られません')).toBeInTheDocument()
    // 何度読み直しても同じなので、再読み込みは出さない
    expect(screen.queryByRole('button', { name: '再読み込み' })).not.toBeInTheDocument()
    unmount()

    respond(() => [500, {}])
    render(<AxisBreakdownPanel matchId={10} />)
    expect(await screen.findByText('内訳を読み込めませんでした。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '再読み込み' })).toBeInTheDocument()
  })

  test('読み込みの失敗後に取り直すと、内訳を出す', async () => {
    let ok = false
    respond(() => (ok ? [200, BREAKDOWN] : [500, {}]))
    render(<AxisBreakdownPanel matchId={10} />)
    expect(await screen.findByText('内訳を読み込めませんでした。')).toBeInTheDocument()
    ok = true
    fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
    expect(await screen.findByRole('table')).toBeInTheDocument()
  })

  test('推薦を切り替えたあとに前の推薦の遅い応答が届いても、表示を上書きしない', async () => {
    const pending: { url: string; resolve: (body: unknown) => void }[] = []
    global.fetch = jest.fn(
      (url: RequestInfo | URL) =>
        new Promise((resolve) => pending.push({ url: String(url), resolve: (body) => resolve({ ok: true, status: 200, json: async () => body }) })),
    ) as unknown as typeof fetch
    const { rerender } = render(<AxisBreakdownPanel matchId={10} />)
    await waitFor(() => expect(pending).toHaveLength(1))
    rerender(<AxisBreakdownPanel matchId={11} />)
    await waitFor(() => expect(pending).toHaveLength(2))
    expect(pending[1].url).toBe('/api/chat/matches/11/axes')

    await act(async () => pending[1].resolve({ ...BREAKDOWN, match_id: 11, strengths: [{ key: 'technical', text: '推薦11の文' }] }))
    expect(await screen.findByText('推薦11の文')).toBeInTheDocument()
    await act(async () => pending[0].resolve({ ...BREAKDOWN, strengths: [{ key: 'technical', text: '推薦10の文' }] }))
    expect(screen.queryByText('推薦10の文')).not.toBeInTheDocument()
    expect(screen.getByText('推薦11の文')).toBeInTheDocument()
  })

  test('取得済みの内訳は、開き直しても取り直さない', async () => {
    const urls = respond(() => [200, BREAKDOWN])
    const cache = new Map<number, AxisBreakdown>()
    const { unmount } = render(<AxisBreakdownPanel matchId={10} cache={cache} />)
    await screen.findByRole('table')
    unmount()
    render(<AxisBreakdownPanel matchId={10} cache={cache} />)
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(urls.filter((u) => u.includes('/axes'))).toHaveLength(1)
  })
})

describe('結果一覧への組み込み', () => {
  const company: Company = {
    id: '1', matchId: 10, name: 'サンプル株式会社', industry: 'IT', location: '東京都', employees: '100名',
    description: '', matchScore: 92, tags: [], techStack: [],
  }
  const noop = () => {}

  test('「軸ごとの内訳を見る」で開いたときだけ取りに行き、企業詳細は開かない', async () => {
    const urls = respond((url) => (url.includes('/axes') ? [200, BREAKDOWN] : [200, []]))
    const onSelectCompany = jest.fn()
    render(
      <ResultsListView
        companies={[company]} sessionId="s1"
        favoriteCompanies={[]} favoritesLoading={false} favoritesError={null} onRetryFavorites={noop}
        isProvisional={false} analysisScores={null} scoreComment="" analysisError={null}
        onRetryAnalysis={noop} jobSuitabilityComment="" suggestedRoles={[]} emailSending={false} favoritingId={null} applyingId={null}
        snackbar={{ open: false, message: '', severity: 'success' }} isGuestUser={false} applyEnabled onCloseSnackbar={noop}
        onBack={noop} onReset={noop} onSendEmail={noop} onSelectCompany={onSelectCompany} onToggleFavorite={noop} onUnfavoriteCompany={noop} onApply={noop} onNavigate={noop}
      />,
    )
    expect(screen.getByText('あなたの10の傾向')).toBeInTheDocument()
    await waitFor(() => expect(urls).toContain('/api/chat/axis-scores?session_id=s1'))
    expect(urls.some((u) => u.includes('/axes'))).toBe(false)

    const toggle = screen.getByRole('button', { name: '軸ごとの内訳を見る' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(toggle)
    expect(await screen.findByRole('table')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '軸ごとの内訳を閉じる' })).toHaveAttribute('aria-expanded', 'true')
    expect(onSelectCompany).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: '軸ごとの内訳を閉じる' }))
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  function renderList(c: Company, onSelectCompany = jest.fn()) {
    render(
      <ResultsListView
        companies={[c]} sessionId="s1"
        favoriteCompanies={[]} favoritesLoading={false} favoritesError={null} onRetryFavorites={noop}
        isProvisional={false} analysisScores={null} scoreComment="" analysisError={null}
        onRetryAnalysis={noop} jobSuitabilityComment="" suggestedRoles={[]} emailSending={false} favoritingId={null} applyingId={null}
        snackbar={{ open: false, message: '', severity: 'success' }} isGuestUser={false} applyEnabled onCloseSnackbar={noop}
        onBack={noop} onReset={noop} onSendEmail={noop} onSelectCompany={onSelectCompany} onToggleFavorite={noop} onUnfavoriteCompany={noop} onApply={noop} onNavigate={noop}
      />,
    )
    return onSelectCompany
  }

  test('Enter キーで内訳を開いても、行のキー操作として企業詳細を開かない', async () => {
    respond((url) => (url.includes('/axes') ? [200, BREAKDOWN] : [200, []]))
    const onSelectCompany = renderList(company)
    const toggle = screen.getByRole('button', { name: '軸ごとの内訳を見る' })
    fireEvent.keyDown(toggle, { key: 'Enter', code: 'Enter' })
    fireEvent.click(toggle)
    expect(await screen.findByRole('table')).toBeInTheDocument()
    expect(onSelectCompany).not.toHaveBeenCalled()
  })

  test('内訳を開いた行では「噛み合った軸」を隠す（同じ軸が別の値で並ばない）。閉じて開き直しても取り直さない', async () => {
    const urls = respond((url) => (url.includes('/axes') ? [200, BREAKDOWN] : [200, []]))
    renderList({ ...company, categoryScores: { technical: 97, teamwork: 50, leadership: 50, creativity: 50, stability: 50, growth: 50, work_life: 50, challenge: 50, detail: 50, communication: 50 } })
    expect(screen.getByText(/噛み合った軸：/)).toHaveTextContent('技術力 97')
    fireEvent.click(screen.getByRole('button', { name: '軸ごとの内訳を見る' }))
    await screen.findByRole('table')
    expect(screen.queryByText(/噛み合った軸：/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '軸ごとの内訳を閉じる' }))
    expect(screen.getByText(/噛み合った軸：/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '軸ごとの内訳を見る' }))
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(urls.filter((u) => u.includes('/axes'))).toHaveLength(1)
  })
})
