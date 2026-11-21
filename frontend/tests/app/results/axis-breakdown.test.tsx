/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
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
    { key: 'technical', label: '技術志向', user_score: 72, company_weight: 80, match: 92 },
    { key: 'growth', label: '成長志向', user_score: null, company_weight: 70, match: null },
  ],
  strengths: [{ key: 'technical', text: 'あなたの「技術志向」（72）は、この企業が重視している度合い（80）と近く、一致しています。' }],
  gaps: [{ key: 'stability', text: 'この企業は「安定志向」を重視しています（80）。あなたの今のスコアは 40 です。関係する経験を具体的に話せるようにすると、伝わりやすくなります。' }],
  unmeasured: ['成長志向'],
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

test('10軸のスコアを固定の並びにそろえ、無い軸・数値でない値は未計測（null）にする', () => {
  const got = toMyCategoryScores([
    { key: 'stability', label: '安定志向', score: 41.6 },
    { key: 'technical', label: '技術志向', score: 80 },
    { key: 'teamwork', label: 'チームワーク志向', score: null },
  ])
  expect(got).toHaveLength(10)
  expect(got[0]).toEqual({ label: '技術志向', score: 80 })
  expect(got[4]).toEqual({ label: '安定志向', score: 42 })
  expect(got[1]).toEqual({ label: 'チームワーク志向', score: null })
})

describe('あなたの10の傾向', () => {
  test('10軸を固定の並びで、数値も文字で出す。測れていない軸は「未計測」', async () => {
    const urls = respond(() => [200, { axes: [{ key: 'technical', label: '技術志向', score: 80 }] }])
    render(<MyCategoryScoresCard sessionId="s 1" />)
    expect(await screen.findByRole('img', { name: '技術志向 80' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'チームワーク志向 未計測' })).toBeInTheDocument()
    expect(screen.getAllByText('未計測')).toHaveLength(9)
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
    respond(() => (ok ? [200, { axes: [{ key: 'technical', label: '技術志向', score: 80 }] }] : [500, {}]))
    render(<MyCategoryScoresCard sessionId="s1" />)
    expect(await screen.findByText('傾向のスコアを読み込めませんでした。')).toBeInTheDocument()
    ok = true
    fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
    expect(await screen.findByRole('img', { name: '技術志向 80' })).toBeInTheDocument()
  })
})

describe('軸ごとの内訳', () => {
  test('軸ごとの数値と、合っている点・補う点・未計測の軸を出す', async () => {
    const urls = respond(() => [200, BREAKDOWN])
    render(<AxisBreakdownPanel matchId={10} />)
    const table = await screen.findByRole('table')
    const tech = within(table).getByRole('row', { name: /技術志向/ })
    expect(within(tech).getByText('72')).toBeInTheDocument()
    expect(within(tech).getByText('80')).toBeInTheDocument()
    expect(within(tech).getByText('92')).toBeInTheDocument()
    expect(within(within(table).getByRole('row', { name: /成長志向/ })).getByText('未計測')).toBeInTheDocument()

    expect(screen.getByText('合っている点')).toBeInTheDocument()
    expect(screen.getByText(/「技術志向」（72）は/)).toBeInTheDocument()
    expect(screen.getByText('補うと伝わりやすくなる点')).toBeInTheDocument()
    expect(screen.getByText(/「成長志向」は、まだ診断で測れていません/)).toBeInTheDocument()
    expect(screen.getByText(/推薦の順位を決めるものではありません/)).toBeInTheDocument()
    expect(urls[0]).toBe('/api/chat/matches/10/axes')
  })

  test('見られない推薦（404）と、読み込みの失敗を区別して知らせる', async () => {
    respond(() => [404, {}])
    const { unmount } = render(<AxisBreakdownPanel matchId={10} />)
    expect(await screen.findByText('この企業の内訳は見られません')).toBeInTheDocument()
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
})
