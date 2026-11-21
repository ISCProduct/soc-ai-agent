/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor, within, act } from '@testing-library/react'
import CompanySearchPage from '@/app/companies/page-content'
import { buildSearchParams } from '@/lib/company-search'

// SOCAIAGENT-612: 学生向けの企業検索。

jest.mock('@/lib/auth', () => ({
  authService: { getUserFetchHeaders: () => ({ 'X-User-Token': 'jwt' }) },
}))

const OPTIONS = { industries: ['IT', '製造業'], prefectures: ['東京都', '大阪府'], job_categories: [{ id: 3, name: 'エンジニア' }] }
const company = (id: number, name: string) => ({ id, name, industry: 'IT', location: '東京都渋谷区', description: `${name}の説明`, logo_url: '' })

type Handler = (url: string, init?: RequestInit) => [number, unknown] | Promise<[number, unknown]>
function mockFetch(handler: Handler) {
  const calls: { url: string; init?: RequestInit }[] = []
  global.fetch = jest.fn(async (url: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(url), init })
    const [status, body] = await handler(String(url), init)
    return { ok: status < 400, status, json: async () => body }
  }) as unknown as typeof fetch
  return calls
}

const defaultRoutes: Handler = (url) => {
  if (url.startsWith('/api/companies/search-options')) return [200, OPTIONS]
  if (url.startsWith('/api/companies/search/favorites')) return [200, { favorites: [{ company_id: 2, match_id: 50, is_favorited: false }] }]
  if (url.startsWith('/api/chat/favorite')) return [200, { ok: true }]
  return [200, { companies: [company(1, 'A社'), company(2, 'B社')], total: 2 }]
}

test('検索の条件をクエリにする（ページは offset に直す）', () => {
  expect(buildSearchParams({ name: ' A社 ', industry: 'IT', prefecture: '東京都', jobCategoryId: 3, page: 3 }).toString())
    .toBe('name=A%E7%A4%BE&industry=IT&prefecture=%E6%9D%B1%E4%BA%AC%E9%83%BD&job_category_id=3&limit=20&offset=40')
  expect(buildSearchParams({ page: 1 }).toString()).toBe('limit=20&offset=0')
})

test('企業を名前順の一覧で出し、企業詳細へのリンクを付ける。並びがおすすめ順でないことを書く', async () => {
  mockFetch(defaultRoutes)
  render(<CompanySearchPage />)
  const items = await screen.findAllByRole('listitem')
  expect(within(items[0]).getByRole('link')).toHaveAttribute('href', '/company/1')
  expect(screen.getByText('2社')).toBeInTheDocument()
  expect(screen.getByText(/おすすめ順ではありません/)).toBeInTheDocument()
})

test('業界・地域・職種を選ぶと、その条件で1ページ目から取り直す', async () => {
  const calls = mockFetch(defaultRoutes)
  render(<CompanySearchPage />)
  await screen.findAllByRole('listitem')
  await waitFor(() => expect(screen.getByLabelText('地域')).not.toHaveAttribute('aria-disabled', 'true'))

  fireEvent.mouseDown(screen.getByLabelText('地域'))
  fireEvent.click(await screen.findByRole('option', { name: '大阪府' }))
  await waitFor(() => expect(calls.some((c) => c.url.includes('prefecture=%E5%A4%A7%E9%98%AA%E5%BA%9C') && c.url.includes('offset=0'))).toBe(true))

  fireEvent.change(screen.getByLabelText('企業名'), { target: { value: 'B社' } })
  fireEvent.click(screen.getByRole('button', { name: '検索' }))
  await waitFor(() => expect(calls.some((c) => c.url.includes('name=B%E7%A4%BE'))).toBe(true))
  expect(screen.getByRole('button', { name: '条件をクリア' })).toBeInTheDocument()
})

test('前の条件の遅い応答が後から届いても、一覧を上書きしない', async () => {
  const pending: ((v: [number, unknown]) => void)[] = []
  mockFetch((url) => {
    if (url.startsWith('/api/companies/search-options')) return [200, OPTIONS]
    if (url.startsWith('/api/companies/search/favorites')) return [200, { favorites: [] }]
    return new Promise((resolve) => pending.push(resolve))
  })
  render(<CompanySearchPage />)
  await waitFor(() => expect(pending).toHaveLength(1))
  fireEvent.change(screen.getByLabelText('企業名'), { target: { value: '新しい条件' } })
  fireEvent.click(screen.getByRole('button', { name: '検索' }))
  await waitFor(() => expect(pending).toHaveLength(2))

  await act(async () => pending[1]([200, { companies: [company(9, '新しい条件の企業')], total: 1 }]))
  expect(await screen.findByText('新しい条件の企業')).toBeInTheDocument()
  await act(async () => pending[0]([200, { companies: [company(1, '古い条件の企業')], total: 1 }]))
  expect(screen.queryByText('古い条件の企業')).not.toBeInTheDocument()
})

test('お気に入りは推薦にある企業だけハートを出し、切り替えられる', async () => {
  const calls = mockFetch(defaultRoutes)
  render(<CompanySearchPage />)
  const items = await screen.findAllByRole('listitem')
  const heart = await within(items[1]).findByRole('button', { name: 'B社をお気に入りに入れる' })
  expect(within(items[0]).queryByRole('button', { name: /お気に入り/ })).not.toBeInTheDocument()
  expect(calls.some((c) => c.url === '/api/companies/search/favorites?company_ids=1,2')).toBe(true)

  fireEvent.click(heart)
  expect(await within(items[1]).findByRole('button', { name: 'B社をお気に入りから外す' })).toHaveAttribute('aria-pressed', 'true')
  const toggle = calls.find((c) => c.url === '/api/chat/favorite')
  expect(toggle?.init?.body).toBe(JSON.stringify({ match_id: 50 }))
})

test('お気に入りの状態が取れなくても一覧は出し、失敗と再試行を出す（推薦が無いのと区別する）', async () => {
  let favOk = false
  mockFetch((url, init) =>
    url.startsWith('/api/companies/search/favorites') && !favOk ? [500, {}] : defaultRoutes(url, init),
  )
  render(<CompanySearchPage />)
  expect(await screen.findByText('A社')).toBeInTheDocument()
  expect(await screen.findByText(/お気に入りの状態を読み込めませんでした/)).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /お気に入り/ })).not.toBeInTheDocument()

  favOk = true
  fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
  expect(await screen.findByRole('button', { name: 'B社をお気に入りに入れる' })).toBeInTheDocument()
  expect(screen.queryByText(/お気に入りの状態を読み込めませんでした/)).not.toBeInTheDocument()
})

test('お気に入りの送信中に同じ企業を続けて押しても、二重に送らない', async () => {
  let release: () => void = () => {}
  const calls = mockFetch((url, init) =>
    url.startsWith('/api/chat/favorite')
      ? new Promise<[number, unknown]>((resolve) => { release = () => resolve([200, { ok: true }]) })
      : defaultRoutes(url, init),
  )
  render(<CompanySearchPage />)
  const heart = await screen.findByRole('button', { name: 'B社をお気に入りに入れる' })
  fireEvent.click(heart)
  fireEvent.click(heart)
  await act(async () => { release() })
  expect(await screen.findByRole('button', { name: 'B社をお気に入りから外す' })).toBeInTheDocument()
  expect(calls.filter((c) => c.url === '/api/chat/favorite')).toHaveLength(1)
})

test('AI が補完した企業には「AI推定」の印を付ける（公的DB由来には付けない）', async () => {
  mockFetch((url, init) =>
    url.startsWith('/api/companies/search?')
      ? [200, { companies: [{ ...company(1, 'A社'), source_type: 'web_search', last_fetch_confidence: 'low' }, { ...company(2, 'B社'), source_type: 'gbizinfo' }], total: 2 }]
      : defaultRoutes(url, init),
  )
  render(<CompanySearchPage />)
  const items = await screen.findAllByRole('listitem')
  expect(within(items[0]).getByText(/AI推定/)).toBeInTheDocument()
  expect(within(items[1]).queryByText(/AI推定/)).not.toBeInTheDocument()
})

test('0件・読み込みの失敗・選択肢の失敗を出し分ける', async () => {
  mockFetch((url, init) => (url.startsWith('/api/companies/search?') ? [200, { companies: [], total: 0 }] : defaultRoutes(url, init)))
  const { unmount } = render(<CompanySearchPage />)
  expect(await screen.findByText('まだ企業が登録されていません。')).toBeInTheDocument()
  unmount()

  let ok = false
  mockFetch((url, init) => {
    if (url.startsWith('/api/companies/search-options')) return [500, {}]
    if (url.startsWith('/api/companies/search?')) return ok ? [200, { companies: [company(1, 'A社')], total: 1 }] : [500, {}]
    return defaultRoutes(url, init)
  })
  render(<CompanySearchPage />)
  expect(await screen.findByText(/企業を読み込めませんでした/)).toBeInTheDocument()
  expect(screen.getByText(/絞り込みの選択肢を読み込めませんでした/)).toBeInTheDocument()
  ok = true
  fireEvent.click(screen.getAllByRole('button', { name: '再読み込み' })[1])
  expect(await screen.findByText('A社')).toBeInTheDocument()
})

test('21社以上ならページを切り替えられる', async () => {
  const calls = mockFetch((url, init) => (url.startsWith('/api/companies/search?') ? [200, { companies: [company(1, 'A社')], total: 45 }] : defaultRoutes(url, init)))
  render(<CompanySearchPage />)
  await screen.findByText('A社')
  fireEvent.click(screen.getByRole('button', { name: 'Go to page 3' }))
  await waitFor(() => expect(calls.some((c) => c.url.includes('offset=40'))).toBe(true))
})
