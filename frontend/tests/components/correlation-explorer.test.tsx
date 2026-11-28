/** @jest-environment jsdom */
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import CorrelationExplorer from '@/components/diagram/explorer/CorrelationExplorer'
import { CompanyGraph } from '@/components/diagram/explorer/CompanyGraph'
import { CompanyDetails } from '@/components/diagram/explorer/CompanyDetails'
import { fetchCorrelationCompare, fetchCorrelationMap } from '@/lib/company/correlation-map'
import { sampleMap, subsidiary } from '../fixtures/correlation-map'

jest.mock('@/lib/company/correlation-map', () => ({
  ...jest.requireActual('@/lib/company/correlation-map'), fetchCorrelationMap: jest.fn(), fetchCorrelationCompare: jest.fn(),
}))
const mockFetchMap = jest.mocked(fetchCorrelationMap)
const mockFetchCompare = jest.mocked(fetchCorrelationCompare)

test.each([
  { initial: 'capital', mode: 'business', tab: '取引・協力関係', target: 10, name: 'あおば取引先' },
  { initial: 'business', mode: 'capital', tab: '親会社・子会社', target: 4, name: 'あおばクラウド' },
] as const)('following a company preserves the current $mode tab after switching tabs', async ({ initial, mode, tab, target, name }) => {
  const onFocus = jest.fn()
  const { rerender } = render(<CorrelationExplorer companyId={1} initialMode={initial} onFocus={onFocus} />)
  await screen.findByRole('tab', { name: tab })
  screen.getByRole('tabpanel').scrollTo = jest.fn()
  fireEvent.click(screen.getByRole('tab', { name: tab }))
  fireEvent.click(screen.getByRole('button', { name: `${name}の情報を見る` }))
  const details = await screen.findByRole('dialog', { name })
  fireEvent.click(within(details).getByRole('button', { name: 'この企業からつながりを探す' }))
  expect(onFocus).toHaveBeenCalledWith(target, mode)
  mockFetchMap.mockResolvedValue({ ...sampleMap(), focus_company_id: target })
  rerender(<CorrelationExplorer companyId={target} initialMode={mode} onFocus={onFocus} />)
  expect(await screen.findByRole('tab', { name: tab })).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByRole('button', { name: '青葉テクノロジーの情報を見る' })).toBeInTheDocument()
})

test.each([true, false])('details limit relationships and reveal comparisons on demand (mobile: %s)', async (mobile) => {
  const original = window.matchMedia
  window.matchMedia = jest.fn((query: string) => ({ matches: mobile && query.includes('max-width:599.95px'), media: query, onchange: null,
    addListener: jest.fn(), removeListener: jest.fn(), addEventListener: jest.fn(), removeEventListener: jest.fn(), dispatchEvent: jest.fn() }))
  try {
    const map = sampleMap()
    map.capital_edges.push(subsidiary(4, 7))
    mockFetchCompare.mockResolvedValue({ focus_company_id: 1, selected_company_id: 4, focus: map.nodes[0], selected: map.nodes[3], compare_text_available: true })
    render(<CompanyDetails node={map.nodes[3]} map={map} mode="capital" onClose={jest.fn()} onFocus={jest.fn()} />)
    expect(screen.queryByText('あおば保守は、あおばクラウドの子会社として登録されています。')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '残りのつながりを見る（1件）' }))
    expect(screen.getByText('あおば保守は、あおばクラウドの子会社として登録されています。')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'つながりを折りたたむ' }))
    expect(screen.queryByText('あおば保守は、あおばクラウドの子会社として登録されています。')).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '青葉テクノロジー', level: 4 })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '起点の企業と比較する' }))
    expect(await screen.findByRole('heading', { name: '青葉テクノロジー', level: 4 })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '閉じる' })).toBeInTheDocument()
  } finally { window.matchMedia = original }
})
test.each(['capital', 'business'] as const)('an empty %s graph keeps the origin without an empty-relationship notice', (mode) => {
  const map = sampleMap()
  map.capital_edges = []
  map.business_edges = []
  render(<CompanyGraph map={map} mode={mode} selectedId={null} onSelect={jest.fn()} onFocus={jest.fn()} all={false} filtered={null} />)
  expect(screen.getByRole('button', { name: '青葉テクノロジーの情報を見る（起点）' })).toBeInTheDocument()
  expect(screen.queryByText(/まだ登録されていません/)).not.toBeInTheDocument()
})
beforeEach(() => {
  mockFetchMap.mockReset().mockResolvedValue(sampleMap())
  mockFetchCompare.mockReset().mockImplementation(async (focus, selected) => ({
    focus_company_id: focus, selected_company_id: selected, compare_text_available: false,
    focus: sampleMap().nodes.find((node) => node.id === focus)!, selected: sampleMap().nodes.find((node) => node.id === selected)!,
  }))
})
test('keeps the parent, initially shows four subsidiaries and expands without losing existing cards', async () => {
  render(<CorrelationExplorer companyId={1} onFocus={jest.fn()} />)
  expect(await screen.findByRole('button', { name: '青葉ホールディングスの情報を見る' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'あおば保守の情報を見る' })).not.toBeInTheDocument()
  const originalCard = screen.getByRole('button', { name: 'あおばアプリの情報を見る' })
  expect(screen.getByText('直接つながる子会社：5社中4社を表示')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '残り1社を見る' }))
  expect(screen.getByRole('button', { name: 'あおば保守の情報を見る' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'あおばアプリの情報を見る' })).toBe(originalCard)
  expect(screen.getByText('直接つながる子会社：5社中5社を表示')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '表示を少なくする' }))
  expect(screen.queryByRole('button', { name: 'あおば保守の情報を見る' })).not.toBeInTheDocument()
  expect(screen.getByText('直接つながる子会社：5社中4社を表示')).toBeInTheDocument()
})
test('a tag finds a folded grandchild and clearing restores the diagram', async () => {
  render(<CorrelationExplorer companyId={1} onFocus={jest.fn()} />)
  const tag = await screen.findByRole('button', { name: 'クラウド' })
  expect(screen.queryByRole('button', { name: '東日本クラウドの情報を見る' })).not.toBeInTheDocument()
  fireEvent.click(tag)
  expect(screen.getByRole('button', { name: '東日本クラウドの情報を見る' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '企業を一覧で見る' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'あおばクラウドの情報を見る' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'あおばアプリの情報を見る' })).not.toBeInTheDocument()
  const grandchild = screen.getByRole('button', { name: '東日本クラウドの情報を見る' }).closest('li')!
  expect(grandchild).toHaveAttribute('data-edge-from', '4')
  expect(grandchild).toHaveAttribute('data-edge-to', '9')
  expect(grandchild.parentElement!.closest('li')).toHaveAttribute('data-edge-from', '1')
  expect(screen.getByRole('button', { name: '拡大表示' })).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'すべて' }))
  expect(screen.queryByRole('button', { name: '東日本クラウドの情報を見る' })).not.toBeInTheDocument()
})

test('mobile shows every direct company without a count toggle in either relationship mode', async () => {
  const original = window.matchMedia
  window.matchMedia = jest.fn((query: string) => ({
    matches: query.includes('max-width:599.95px'), media: query, onchange: null,
    addListener: jest.fn(), removeListener: jest.fn(), addEventListener: jest.fn(), removeEventListener: jest.fn(), dispatchEvent: jest.fn(),
  }))
  try {
    const map = sampleMap()
    map.business_edges = [3, 4, 5, 6, 7].map((id) => ({ ...map.business_edges[0], id, from_id: 1, to_id: id }))
    mockFetchMap.mockResolvedValue(map)
    render(<CorrelationExplorer companyId={1} onFocus={jest.fn()} />)
    await screen.findByRole('tabpanel')
    screen.getByRole('tabpanel').scrollTo = jest.fn()
    expect(screen.getByRole('button', { name: 'あおば保守の情報を見る' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /残り.*社を見る|表示を少なくする/ })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: '取引・協力関係' }))
    expect(screen.getByRole('button', { name: 'あおば保守の情報を見る' })).toBeInTheDocument()
    expect(screen.queryByText(/直接つながる/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /残り.*社を見る|表示を少なくする/ })).not.toBeInTheDocument()
  } finally {
    window.matchMedia = original
  }
})
test('mobile details explain the relationship, compare actual businesses and retain the details link', async () => {
  render(<CorrelationExplorer companyId={1} onFocus={jest.fn()} />)
  fireEvent.click(await screen.findByRole('button', { name: 'あおばクラウドの情報を見る' }))
  const sheet = await screen.findByRole('dialog')
  expect(within(sheet).getAllByText(/事業上の役割や協力内容はまだ登録/)).toHaveLength(2)
  fireEvent.click(within(sheet).getByRole('button', { name: '起点の企業と比較する' }))
  expect(await within(sheet).findByRole('heading', { name: '青葉テクノロジー', level: 4 })).toBeInTheDocument()
  expect(within(sheet).getByRole('link', { name: '企業詳細を見る' })).toHaveAttribute('href', '/company/4')
  const ai = within(sheet).getByRole('button', { name: 'AIに質問する' })
  expect(ai).toBeEnabled()
  expect(screen.queryByText('AIチャットは準備中です')).not.toBeInTheDocument()
  fireEvent.click(ai)
  const notice = await screen.findByRole('dialog', { name: '企業研究AI' })
  expect(within(notice).getByText('AIチャットは準備中です')).toBeVisible()
  expect(screen.getAllByRole('dialog')).toHaveLength(1)
  expect(within(notice).queryByRole('link', { name: '企業詳細を見る' })).not.toBeInTheDocument()
  expect(within(notice).getByRole('heading', { name: '企業研究AI' })).toHaveFocus()
  expect(within(sheet).queryByText('100%')).not.toBeInTheDocument()
  fireEvent.click(within(notice).getByRole('button', { name: '閉じる' }))
})

test('filtered graphs preserve upstream edge direction and terminate cycles', () => {
  const map = sampleMap()
  map.capital_edges.push(subsidiary(9, 1))
  render(<CompanyGraph map={map} mode="capital" selectedId={null} onSelect={jest.fn()} onFocus={jest.fn()} all={false} filtered={[map.nodes[1], map.nodes[8]]} />)
  const parent = screen.getByRole('button', { name: '青葉ホールディングスの情報を見る' }).closest('li')!
  expect(parent).toHaveAttribute('data-edge-from', '2')
  expect(parent).toHaveAttribute('data-edge-to', '1')
  expect(parent).toHaveClass('reverseArrow')
  expect(screen.getAllByRole('button', { name: '青葉テクノロジーの情報を見る（起点）' })).toHaveLength(1)
  expect(screen.getAllByRole('button', { name: '東日本クラウドの情報を見る' })).toHaveLength(1)
})

test('filtered business graphs connect matches without inventing a direction', () => {
  const map = sampleMap()
  render(<CompanyGraph map={map} mode="business" selectedId={null} onSelect={jest.fn()} onFocus={jest.fn()} all={false} filtered={[map.nodes[9]]} />)
  const partner = screen.getByRole('button', { name: 'あおば取引先の情報を見る' }).closest('li')!
  expect(partner).toHaveAttribute('data-edge-from', '10')
  expect(partner).toHaveAttribute('data-edge-to', '1')
  expect(partner).toHaveClass('undirected')
  expect(screen.queryByRole('button', { name: 'あおばアプリの情報を見る' })).not.toBeInTheDocument()
})
test('failed requests expose retry and a late response cannot replace a new origin', async () => {
  mockFetchMap.mockRejectedValueOnce(new Error('読み込み失敗'))
  const { rerender } = render(<CorrelationExplorer companyId={1} onFocus={jest.fn()} />)
  fireEvent.click(await screen.findByRole('button', { name: '再試行' }))
  await screen.findByRole('button', { name: /青葉テクノロジーの情報を見る/ })
  let complete: ((value: ReturnType<typeof sampleMap>) => void) | undefined
  mockFetchMap.mockImplementationOnce(() => new Promise((resolve) => { complete = resolve }))
  rerender(<CorrelationExplorer companyId={3} onFocus={jest.fn()} />)
  mockFetchMap.mockResolvedValueOnce({ ...sampleMap(), focus_company_id: 4 })
  rerender(<CorrelationExplorer companyId={4} onFocus={jest.fn()} />)
  await screen.findByText('あおばクラウドから探す')
  await act(async () => complete?.({ ...sampleMap(), focus_company_id: 3 }))
  expect(screen.queryByText('あおばアプリから探す')).not.toBeInTheDocument()
})
test('cyclic data does not expand recursively into the origin', () => {
  const map = sampleMap()
  map.capital_edges.push(subsidiary(4, 1))
  render(<CompanyGraph map={map} mode="capital" selectedId={null} onSelect={jest.fn()} onFocus={jest.fn()} all={false} filtered={null} />)
  fireEvent.click(screen.getByRole('button', { name: '子会社1社を見る' }))
  expect(screen.getByRole('button', { name: '東日本クラウドの情報を見る' })).toBeInTheDocument()
  expect(screen.getAllByRole('button', { name: /青葉テクノロジーの情報を見る/ })).toHaveLength(1)
})
