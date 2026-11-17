/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import GraduateOutcomesPage from '@/app/graduate-outcomes/page-content'
import { GraduateOutcomeCard } from '@/components/company/GraduateOutcomeCard'

// #2032: 卒業生の進路実績。

jest.mock('@/lib/auth', () => ({
  authService: { getUserFetchHeaders: () => ({ 'X-User-Token': 'jwt' }) },
}))

const LIST = {
  school_linked: true,
  total: 8,
  companies: [
    { company_id: 2, company_name: 'A社', industry: '製造業', total: 6, by_year: [{ year: 2025, count: 3 }, { year: 0, count: 3 }], by_year_few: false },
    { company_id: 3, company_name: 'C社', industry: '製造業', total: 3, by_year: [], by_year_few: true },
    { company_id: 1, company_name: 'B社', industry: '情報通信業', total: null, by_year: [], by_year_few: false },
  ],
  years: [2025, 2024],
  industries: ['情報通信業', '製造業'],
}

function mockFetch(body: unknown, status = 200) {
  const urls: string[] = []
  global.fetch = jest.fn(async (url: RequestInfo | URL) => {
    urls.push(String(url))
    return { ok: status < 400, status, json: async () => body }
  }) as unknown as typeof fetch
  return urls
}

test('企業ごとの人数を多い順に出し、年度で絞り込める', async () => {
  const urls = mockFetch(LIST)
  render(<GraduateOutcomesPage />)
  expect(await screen.findByText('合計 8人・3社')).toBeInTheDocument()
  const items = screen.getAllByRole('listitem')
  expect(within(items[0]).getByText('A社')).toBeInTheDocument()
  expect(within(items[0]).getByText('2025年卒 3人')).toBeInTheDocument()

  fireEvent.mouseDown(screen.getByLabelText('卒業年度'))
  fireEvent.click(await screen.findByRole('option', { name: '2024年卒' }))
  await waitFor(() => expect(urls.at(-1)).toContain('year=2024'))
})

test('3人未満は人数を出さず「少数」、年度が未登録の実績は「年度未登録」と出す', async () => {
  mockFetch(LIST)
  render(<GraduateOutcomesPage />)
  const items = await screen.findAllByRole('listitem')
  expect(within(items[0]).getByText('年度未登録 3人')).toBeInTheDocument()
  expect(within(items[1]).getByText('ほかの年度 少数')).toBeInTheDocument()
  expect(within(items[2]).getByText('少数')).toBeInTheDocument()
  expect(screen.queryByText(/0年卒/)).not.toBeInTheDocument()
})

test('前の条件の遅い応答が後から届いても、今の条件の結果を上書きしない', async () => {
  const pending: { url: string; resolve: (body: unknown) => void }[] = []
  global.fetch = jest.fn(
    (url: RequestInfo | URL) =>
      new Promise((resolve) => pending.push({ url: String(url), resolve: (body) => resolve({ ok: true, status: 200, json: async () => body }) })),
  ) as unknown as typeof fetch
  render(<GraduateOutcomesPage />)
  await waitFor(() => expect(pending).toHaveLength(1))
  pending[0].resolve(LIST)
  await screen.findByText('合計 8人・3社')

  // 2024年 → 2025年と続けて切り替え、2025年の応答が先に、2024年の応答が後から届く
  fireEvent.mouseDown(screen.getByLabelText('卒業年度'))
  fireEvent.click(await screen.findByRole('option', { name: '2024年卒' }))
  await waitFor(() => expect(pending).toHaveLength(2))
  await waitFor(() => expect(screen.queryByRole('listbox')).not.toBeInTheDocument())
  fireEvent.mouseDown(screen.getByRole('combobox', { name: '卒業年度' }))
  fireEvent.click(await screen.findByRole('option', { name: '2025年卒' }))
  await waitFor(() => expect(pending).toHaveLength(3))
  expect(pending[2].url).toContain('year=2025')

  pending[2].resolve({ ...LIST, total: 3, companies: [{ ...LIST.companies[0], company_name: '2025年の企業', total: 3 }] })
  expect(await screen.findByText('2025年の企業')).toBeInTheDocument()
  pending[1].resolve({ ...LIST, total: 4, companies: [{ ...LIST.companies[0], company_name: '2024年の企業', total: 4 }] })
  await new Promise((r) => setTimeout(r, 0))
  expect(screen.queryByText('2024年の企業')).not.toBeInTheDocument()
  expect(screen.getByText('2025年の企業')).toBeInTheDocument()
})

test('読み込みに失敗したらエラーと再読み込みを出し、再読み込みで取り直す', async () => {
  const urls = mockFetch({}, 500)
  render(<GraduateOutcomesPage />)
  expect(await screen.findByText(/進路実績を読み込めませんでした/)).toBeInTheDocument()
  mockFetch(LIST)
  fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
  expect(await screen.findByText('合計 8人・3社')).toBeInTheDocument()
  expect(urls).toHaveLength(1)
})

test('学校が未登録なら、実績を出さずに案内する', async () => {
  mockFetch({ ...LIST, school_linked: false, companies: [], total: 0 })
  render(<GraduateOutcomesPage />)
  expect(await screen.findByText(/学校が登録されていないため/)).toBeInTheDocument()
})

test('企業詳細のカードは、実績があるときだけ人数を出す', async () => {
  mockFetch({ school_linked: true, total: 4, by_year: [{ year: 2025, count: 4 }], by_year_few: false, by_position: [{ title: 'SE', count: 3 }], by_position_few: true })
  const { unmount } = render(<GraduateOutcomeCard companyId="7" />)
  expect(await screen.findByText('4人')).toBeInTheDocument()
  expect(screen.getByText('SE 3人、ほかの職種 少数')).toBeInTheDocument()
  unmount()

  mockFetch({ school_linked: true, total: 0, by_year: [], by_year_few: false, by_position: [], by_position_few: false })
  const { container } = render(<GraduateOutcomeCard companyId="7" />)
  await waitFor(() => expect(container).toBeEmptyDOMElement())
})

test('企業詳細のカードは、3人未満なら「少数」とだけ出す', async () => {
  mockFetch({ school_linked: true, total: null, by_year: [], by_year_few: false, by_position: [], by_position_few: false })
  render(<GraduateOutcomeCard companyId="7" />)
  expect(await screen.findByText('少数（3人未満）')).toBeInTheDocument()
  expect(screen.queryByText(/年卒/)).not.toBeInTheDocument()
})

test('企業詳細のカードは、企業が見つからなければ出さず、失敗なら再読み込みを出す', async () => {
  mockFetch({ error: 'not found' }, 404)
  const { container, unmount } = render(<GraduateOutcomeCard companyId="7" />)
  await waitFor(() => expect(container).toBeEmptyDOMElement())
  unmount()

  mockFetch({}, 500)
  render(<GraduateOutcomeCard companyId="7" />)
  expect(await screen.findByText('先輩の進路実績を読み込めませんでした')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '再読み込み' })).toBeInTheDocument()
})
