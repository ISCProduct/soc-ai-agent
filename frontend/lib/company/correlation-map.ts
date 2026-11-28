import { z } from 'zod'

const id = z.number().int().positive()
const ids = z.array(id).nullable().transform((value) => value ?? [])
export const correlationNodeSchema = z.object({
  id, name: z.string().min(1), name_reading: z.string(), name_reading_missing: z.boolean(),
  short_business: z.string(), short_business_known: z.boolean(), short_business_source: z.string().optional(),
  main_business: z.string(), description: z.string(), industry: z.string().optional(),
  business_tags: z.array(z.string()).nullable().transform((value) => value ?? []), business_tags_status: z.string(),
  market_type: z.string(), market_type_known: z.boolean(), is_listed: z.boolean(),
  source_url: z.string().optional(), source_fetched_at: z.string().nullish(), relations_fetched_at: z.string().nullish(),
  is_focus: z.boolean(), detail_path: z.string(),
})
const capitalEdgeSchema = z.object({
  id, parent_id: id, child_id: id, relation_type: z.string(), relation_label: z.string(),
  description: z.string(), description_known: z.boolean(), ratio: z.number().nullish(),
})
const businessEdgeSchema = z.object({
  id, from_id: id, to_id: id, direction_known: z.boolean(), relation_type: z.string(),
  relation_label: z.string(), description: z.string(), description_known: z.boolean(),
})
export const correlationMapSchema = z.object({
  focus_company_id: id, nodes: z.array(correlationNodeSchema),
  capital_edges: z.array(capitalEdgeSchema).nullable().transform((value) => value ?? []),
  business_edges: z.array(businessEdgeSchema).nullable().transform((value) => value ?? []),
  meta: z.object({
    direct_parent_ids: ids, direct_child_ids: ids, direct_child_total: z.number(),
    initial_child_limit: z.number(), initial_child_ids: ids, remaining_child_count: z.number(),
    truncated: z.boolean(), max_depth: z.number(), max_nodes: z.number(),
  }),
})
const comparedCompany = correlationNodeSchema.pick({
  id: true, name: true, short_business: true, short_business_known: true,
  short_business_source: true, main_business: true, description: true,
})
export const correlationCompareSchema = z.object({
  focus_company_id: id, selected_company_id: id, focus: comparedCompany, selected: comparedCompany,
  compare_text_available: z.boolean(),
})
export type CorrelationNode = z.infer<typeof correlationNodeSchema>
export type CorrelationMap = z.infer<typeof correlationMapSchema>
export type CorrelationCompare = z.infer<typeof correlationCompareSchema>
export type CapitalEdge = z.infer<typeof capitalEdgeSchema>
export type BusinessEdge = z.infer<typeof businessEdgeSchema>
export type CorrelationMode = 'capital' | 'business'

async function getJSON(path: string, signal: AbortSignal): Promise<unknown> {
  const response = await fetch(path, { signal, cache: 'no-store' })
  if (!response.ok) throw new Error(response.status === 404 ? 'この企業の相関図はまだ用意されていません。' : '情報を取得できませんでした。もう一度お試しください。')
  return response.json()
}
export async function fetchCorrelationMap(companyId: number, signal: AbortSignal): Promise<CorrelationMap> {
  const result = correlationMapSchema.parse(await getJSON(`/api/companies/${companyId}/correlation-map`, signal))
  if (result.focus_company_id !== companyId || !result.nodes.some((node) => node.id === companyId)) throw new Error('企業情報を確認できませんでした。もう一度お試しください。')
  return result
}
export async function fetchCorrelationCompare(focusId: number, selectedId: number, signal: AbortSignal): Promise<CorrelationCompare> {
  const result = correlationCompareSchema.parse(await getJSON(`/api/companies/${focusId}/correlation-map/compare?with=${selectedId}`, signal))
  if (result.focus_company_id !== focusId || result.selected_company_id !== selectedId || result.focus.id !== focusId || result.selected.id !== selectedId) throw new Error('比較する企業の情報を確認できませんでした。')
  return result
}

const collator = new Intl.Collator('ja', { sensitivity: 'base', numeric: true })
export function sortCompanies(nodes: CorrelationNode[]): CorrelationNode[] {
  return [...nodes].sort((a, b) => {
    const aMissing = a.name_reading_missing || !a.name_reading.trim()
    const bMissing = b.name_reading_missing || !b.name_reading.trim()
    return Number(aMissing) - Number(bMissing) || collator.compare(aMissing ? a.name : a.name_reading, bMissing ? b.name : b.name_reading) || a.id - b.id
  })
}
function nodesFor(map: CorrelationMap, values: number[]): CorrelationNode[] {
  const wanted = new Set(values)
  return sortCompanies(map.nodes.filter((node) => wanted.has(node.id)))
}
// meta.direct_child_ids also includes affiliates; never call those subsidiaries.
export function childrenOf(map: CorrelationMap, parentId: number): CorrelationNode[] {
  return nodesFor(map, map.capital_edges.filter((edge) => edge.relation_type === 'capital_subsidiary' && edge.parent_id === parentId && edge.child_id !== parentId).map((edge) => edge.child_id))
}
export function parentsOf(map: CorrelationMap, childId: number): CorrelationNode[] {
  return nodesFor(map, map.capital_edges.filter((edge) => edge.relation_type === 'capital_subsidiary' && edge.child_id === childId && edge.parent_id !== childId).map((edge) => edge.parent_id))
}
export function partnersOf(map: CorrelationMap, companyId: number): CorrelationNode[] {
  return nodesFor(map, map.business_edges.flatMap((edge) => edge.from_id === companyId ? [edge.to_id] : edge.to_id === companyId ? [edge.from_id] : []).filter((value) => value !== companyId))
}
export function affiliatesOf(map: CorrelationMap, companyId: number): CorrelationNode[] {
  return nodesFor(map, map.capital_edges.filter((edge) => edge.relation_type !== 'capital_subsidiary').flatMap((edge) => edge.parent_id === companyId ? [edge.child_id] : edge.child_id === companyId ? [edge.parent_id] : []).filter((value) => value !== companyId))
}
export function marketLabel(node: CorrelationNode): string {
  if (!node.market_type_known) return '上場区分未確認'
  const labels: Record<string, string> = { prime: 'プライム', standard: 'スタンダード', growth: 'グロース', unlisted: '非上場' }
  return labels[node.market_type] ?? '上場区分未確認'
}
export function companyTags(node: CorrelationNode): string[] {
  return node.business_tags_status === 'available' ? [...new Set(node.business_tags.filter(Boolean))] : []
}
// 絞り込み候補は該当企業数の多い上位2タグ。同数はタグ名順にして表示を安定させる。
export function topBusinessTags(nodes: CorrelationNode[]): string[] {
  const companiesByTag = new Map<string, Set<number>>()
  for (const node of nodes) {
    for (const tag of companyTags(node)) {
      if (!companiesByTag.has(tag)) companiesByTag.set(tag, new Set())
      companiesByTag.get(tag)!.add(node.id)
    }
  }
  return [...companiesByTag].sort(([a, aIds], [b, bIds]) => bIds.size - aIds.size || collator.compare(a, b))
    .slice(0, 2).map(([tag]) => tag)
}
export function businessText(node: Pick<CorrelationNode, 'main_business' | 'description' | 'short_business' | 'short_business_known'>): string {
  return node.main_business.trim() || node.description.trim() || (node.short_business_known ? node.short_business.trim() : '') || '事業内容はまだ登録されていません。'
}
export function safeSourceURL(value?: string): string | null {
  try { const url = new URL(value ?? ''); return ['http:', 'https:'].includes(url.protocol) ? url.href : null } catch { return null }
}
