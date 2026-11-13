/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import PortfolioPage from '@/app/portfolio/page-content'

// #2030: 自己分析ポートフォリオ。

jest.mock('next/navigation', () => ({ useRouter: () => ({ push: jest.fn() }) }))
jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, name: '山田 花子' }),
    getUserFetchHeaders: () => ({ 'X-User-Token': 'jwt' }),
  },
}))

const DATA = {
  generated_at: '2026-11-13T05:00:00Z',
  sections: ['diagnosis', 'interview', 'es', 'assessment'],
  diagnosis: { updated_at: '2026-11-01T00:00:00Z', scores: [{ category: '技術志向', score: 82 }] },
  interviews: [{ session_id: 3, created_at: '2026-11-10T00:00:00Z', summary_text: '結論から話せています', scores: {}, strengths: ['具体例がある'], improvements: [] }],
  es_reviews: [],
  assessments: [{ created_at: '2026-10-01T00:00:00Z', title: '適性検査A', scores: [{ category: '主体性', score: 70 }] }],
}

let urls: string[] = []
beforeEach(() => {
  urls = []
  global.fetch = jest.fn(async (url: RequestInfo | URL) => {
    urls.push(String(url))
    return { ok: true, status: 200, json: async () => DATA }
  }) as unknown as typeof fetch
})

test('選んだ内容をまとめて表示し、数値も文字で出す', async () => {
  render(<PortfolioPage />)
  expect(await screen.findByText('結論から話せています')).toBeInTheDocument()
  expect(screen.getByRole('img', { name: '技術志向 82点' })).toBeInTheDocument()
  expect(screen.getByText('まだ ES の添削がありません。')).toBeInTheDocument()
  expect(screen.getByText(/山田 花子/)).toBeInTheDocument()
  expect(urls[0]).toContain('sections=diagnosis%2Cinterview%2Ces%2Cassessment')
})

test('内容を外すと、その内容を除いて取り直す。全部外すと選ぶよう促す', async () => {
  render(<PortfolioPage />)
  await screen.findByText('結論から話せています')
  fireEvent.click(screen.getByLabelText('ES 添削'))
  await waitFor(() => expect(urls.at(-1)).toContain('sections=diagnosis%2Cinterview%2Cassessment'))

  for (const label of ['適性の診断', 'AI 面接', '適性検査']) fireEvent.click(screen.getByLabelText(label))
  expect(await screen.findByText('出力する内容を1つ以上選んでください。')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '印刷・PDFで保存' })).toBeDisabled()
})

test('読み込みに失敗したらエラーと再読み込みを出す', async () => {
  global.fetch = jest.fn(async () => ({ ok: false, status: 500, json: async () => ({}) })) as unknown as typeof fetch
  render(<PortfolioPage />)
  expect(await screen.findByText(/ポートフォリオを読み込めませんでした/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '再読み込み' })).toBeInTheDocument()
})
