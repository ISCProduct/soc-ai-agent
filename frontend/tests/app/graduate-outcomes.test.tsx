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
  total: 3,
  companies: [
    { company_id: 2, company_name: 'A社', industry: '製造業', total: 2, by_year: [{ year: 2025, count: 2 }] },
    { company_id: 1, company_name: 'B社', industry: '情報通信業', total: 1, by_year: [{ year: 2024, count: 1 }] },
  ],
  years: [2025, 2024],
  industries: ['情報通信業', '製造業'],
}

function mockFetch(body: unknown, ok = true) {
  const urls: string[] = []
  global.fetch = jest.fn(async (url: RequestInfo | URL) => {
    urls.push(String(url))
    return { ok, status: ok ? 200 : 500, json: async () => body }
  }) as unknown as typeof fetch
  return urls
}

test('企業ごとの人数を多い順に出し、年度で絞り込める', async () => {
  const urls = mockFetch(LIST)
  render(<GraduateOutcomesPage />)
  expect(await screen.findByText('合計 3人・2社')).toBeInTheDocument()
  const items = screen.getAllByRole('listitem')
  expect(within(items[0]).getByText('A社')).toBeInTheDocument()
  expect(within(items[0]).getByText('2025年卒 2人')).toBeInTheDocument()

  fireEvent.mouseDown(screen.getByLabelText('卒業年度'))
  fireEvent.click(await screen.findByRole('option', { name: '2024年卒' }))
  await waitFor(() => expect(urls.at(-1)).toContain('year=2024'))
})

test('学校が未登録なら、実績を出さずに案内する', async () => {
  mockFetch({ ...LIST, school_linked: false, companies: [], total: 0 })
  render(<GraduateOutcomesPage />)
  expect(await screen.findByText(/学校が登録されていないため/)).toBeInTheDocument()
})

test('企業詳細のカードは、実績があるときだけ人数を出す', async () => {
  mockFetch({ school_linked: true, total: 3, by_year: [{ year: 2025, count: 2 }, { year: 2024, count: 1 }], by_position: [{ title: 'SE', count: 3 }] })
  const { unmount } = render(<GraduateOutcomeCard companyId="7" />)
  expect(await screen.findByText('3人')).toBeInTheDocument()
  expect(screen.getByText('SE 3人')).toBeInTheDocument()
  unmount()

  mockFetch({ school_linked: true, total: 0, by_year: [], by_position: [] })
  const { container } = render(<GraduateOutcomeCard companyId="7" />)
  await waitFor(() => expect(container).toBeEmptyDOMElement())
})
