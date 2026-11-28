/** @jest-environment jsdom */
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { HomeRightPanel } from '@/components/home/HomeRightPanel'
import type { StudentGuidance } from '@/lib/teacher-guidance'

jest.mock('next/navigation', () => ({ useRouter: () => ({ push: jest.fn() }), usePathname: () => '/' }))
jest.mock('@/components/home/NextActionsBar', () => ({ loadNextActions: jest.fn() }))
jest.mock('@/components/home/AIBudgetNotice', () => ({ AIBudgetNotice: () => null }))

const { loadNextActions } = require('@/components/home/NextActionsBar') as { loadNextActions: jest.Mock }

const guidance = (id: number, message: string): StudentGuidance =>
  ({ id, message, suggested_industries: [] }) as unknown as StudentGuidance

function setup(overrides: Partial<React.ComponentProps<typeof HomeRightPanel>> = {}) {
  const props = {
    guidances: [] as StudentGuidance[],
    guidanceLoadFailed: false,
    guidanceError: '',
    dismissingIds: new Set<number>(),
    onDismissGuidance: jest.fn(),
    onRetryGuidances: jest.fn(),
    notices: [],
    ...overrides,
  }
  render(<HomeRightPanel {...props} />)
  return props
}

describe('HomeRightPanel（先生から・これからの予定・お知らせ）', () => {
  beforeEach(() => {
    loadNextActions.mockReset()
    loadNextActions.mockResolvedValue([])
  })

  it('何も無いときは、3つの欄がそれぞれの空の文言と次の操作を出す', async () => {
    setup()
    expect(screen.getByRole('heading', { name: '先生から' })).toBeInTheDocument()
    expect(screen.getByText('先生からの案内はありません。')).toBeInTheDocument()
    expect(await screen.findByText('予定はまだありません。')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '予定を追加する' })).toHaveAttribute('href', '/schedule')
    expect(screen.getByText('新しいお知らせはありません。')).toBeInTheDocument()
  })

  it('先生の案内を出し、「読みました」で既読にする処理を呼ぶ', () => {
    const props = setup({ guidances: [guidance(7, 'まずは自己分析の質問に答えてみましょう。')] })
    expect(screen.getByText('まずは自己分析の質問に答えてみましょう。')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '読みました' }))
    expect(props.onDismissGuidance).toHaveBeenCalledWith(7)
  })

  it('広い画面の案内からも、狭い画面と同じリンクで先生に返信できる（#2028）', () => {
    setup({ guidances: [guidance(7, 'まずは自己分析の質問に答えてみましょう。')] })
    expect(screen.getByRole('link', { name: '先生に返信する' })).toHaveAttribute('href', '/consultations?thread=7')
  })

  it('既読にしている最中の案内は、「読みました」を押せない（連打を防ぐ）', () => {
    setup({ guidances: [guidance(7, '案内')], dismissingIds: new Set([7]) })
    expect(screen.getByRole('button', { name: '読みました' })).toBeDisabled()
  })

  it('案内の取得に失敗したら、その欄だけにエラーと再読み込みを出し、ほかの欄は出す', async () => {
    const props = setup({ guidanceLoadFailed: true })
    expect(screen.getByText(/先生からの案内を読み込めませんでした/)).toBeInTheDocument()
    expect(screen.queryByText('先生からの案内はありません。')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
    expect(props.onRetryGuidances).toHaveBeenCalled()
    expect(await screen.findByText('予定はまだありません。')).toBeInTheDocument()
  })

  it('既読にできなかったときの文言を出す', () => {
    setup({ guidances: [guidance(1, '案内')], guidanceError: '案内を閉じられませんでした。時間をおいて再度お試しください' })
    expect(screen.getByText(/案内を閉じられませんでした/)).toBeInTheDocument()
  })

  it('予定を出し、3件より多いときは件数だけ出す', async () => {
    loadNextActions.mockResolvedValue(
      [1, 2, 3, 4, 5].map((n) => ({ key: `k${n}`, text: `予定${n}`, actionLabel: '確認', href: '/schedule' })),
    )
    setup()
    expect(await screen.findByRole('link', { name: '予定1' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '予定3' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '予定4' })).toBeNull()
    expect(screen.getByText('ほか2件')).toBeInTheDocument()
  })

  it('行き先の無い項目はリンクにしない', async () => {
    loadNextActions.mockResolvedValue([{ key: 'k1', text: '行き先の無い項目', actionLabel: '' }])
    setup()
    expect(await screen.findByText('行き先の無い項目')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '行き先の無い項目' })).toBeNull()
  })

  it('予定の取得に失敗したら、予定の欄だけにエラーと再読み込みを出し、再読み込みで取り直す', async () => {
    loadNextActions.mockRejectedValueOnce(new Error('x')).mockResolvedValueOnce([])
    setup()
    expect(await screen.findByText('予定を読み込めませんでした')).toBeInTheDocument()
    expect(screen.getByText('先生からの案内はありません。')).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: '再読み込み' })[0])
    await waitFor(() => expect(screen.getByText('予定はまだありません。')).toBeInTheDocument())
    expect(loadNextActions).toHaveBeenCalledTimes(2)
  })

  it('お知らせがあるときは、空の文言を出さずに、お知らせを出す', () => {
    setup({ notices: [{ key: 'n1', node: <div>履歴書を確認してください</div> }] })
    expect(screen.getByText('履歴書を確認してください')).toBeInTheDocument()
    expect(screen.queryByText('新しいお知らせはありません。')).toBeNull()
  })
})
