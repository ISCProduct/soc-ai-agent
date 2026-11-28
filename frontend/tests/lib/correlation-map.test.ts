import { affiliatesOf, childrenOf, companyTags, correlationMapSchema, fetchCorrelationMap, marketLabel, parentsOf, partnersOf, safeSourceURL, sortCompanies, topBusinessTags } from '@/lib/company/correlation-map'
import { company, sampleMap, subsidiary } from '../fixtures/correlation-map'

afterEach(() => jest.restoreAllMocks())
test('related companies are not subsidiaries, even when backend metadata includes them', () => {
  const map = sampleMap()
  expect(childrenOf(map, 1).map((node) => node.id)).toEqual([3, 4, 5, 6, 7])
  expect(affiliatesOf(map, 1).map((node) => node.id)).toEqual([8])
  expect(parentsOf(map, 1).map((node) => node.id)).toEqual([2])
})
test('duplicates and self-relations do not create extra cards', () => {
  const map = sampleMap()
  map.capital_edges.push(subsidiary(1, 1), subsidiary(1, 3))
  expect(childrenOf(map, 1).map((node) => node.id)).toEqual([3, 4, 5, 6, 7])
})
test('unknown business direction connects both endpoints without inferring provider', () => {
  expect(partnersOf(sampleMap(), 1).map((node) => node.id)).toEqual([10])
  expect(partnersOf(sampleMap(), 10).map((node) => node.id)).toEqual([1])
})
test('reading order is stable; missing readings fall back to names after known readings', () => {
  const nodes = [company(4, 'A', { name_reading_missing: true }), company(3, 'Z', { name_reading: 'カナ' }), company(2, 'Y', { name_reading: 'かな' }), company(1, 'X', { name_reading: 'あおば' })]
  expect(sortCompanies(nodes).map((node) => node.id)).toEqual([1, 2, 3, 4])
})
test('missing market classification and tags do not become non-listed or industry tags', () => {
  const node = company(1, '会社', { market_type: 'unlisted', industry: 'クラウド', business_tags: ['クラウド'] })
  expect(marketLabel(node)).toBe('上場区分未確認')
  expect(companyTags(node)).toEqual([])
  expect(marketLabel({ ...node, market_type_known: true })).toBe('非上場')
})

test('filter tags show the top two by distinct company count, with stable ties', () => {
  const tagged = (id: number, tags: string[]) => company(id, `企業${id}`, { business_tags: tags, business_tags_status: 'available' })
  const nodes = [tagged(1, ['クラウド', 'クラウド', 'セキュリティ']), tagged(2, ['クラウド', '業務システム']),
    tagged(3, ['クラウド', '業務システム', 'アプリ']), tagged(4, ['デザイン']), tagged(4, ['デザイン']), tagged(4, ['デザイン']),
    company(5, '未整備', { business_tags: ['採用しないタグ'] })]
  expect(topBusinessTags(nodes)).toEqual(['クラウド', '業務システム'])
  expect(topBusinessTags([tagged(1, ['デザイン', 'クラウド', 'アプリ'])])).toEqual(['アプリ', 'クラウド'])
  expect(topBusinessTags([...nodes].reverse())).toEqual(topBusinessTags(nodes))
  expect(topBusinessTags([tagged(1, ['クラウド'])])).toEqual(['クラウド'])
  expect(topBusinessTags([])).toEqual([])
})
test('Go nil slices are empty lists, but malformed data is rejected', () => {
  expect(correlationMapSchema.parse({ ...sampleMap(), capital_edges: null }).capital_edges).toEqual([])
  expect(() => correlationMapSchema.parse({ nodes: [] })).toThrow()
})
test('source links cannot execute scripts', () => {
  expect(safeSourceURL('javascript:alert(1)')).toBeNull()
  expect(safeSourceURL('https://example.com/company')).toBe('https://example.com/company')
})
test('map request rejects a different origin and propagates cancellation', async () => {
  const mocked = jest.spyOn(global, 'fetch').mockResolvedValue(new Response(JSON.stringify(sampleMap())))
  const controller = new AbortController()
  await expect(fetchCorrelationMap(20, controller.signal)).rejects.toThrow('企業情報を確認')
  expect(mocked).toHaveBeenCalledWith('/api/companies/20/correlation-map', expect.objectContaining({ signal: controller.signal, cache: 'no-store' }))
})
