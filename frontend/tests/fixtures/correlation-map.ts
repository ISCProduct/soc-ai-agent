import type { CapitalEdge, CorrelationMap, CorrelationNode } from '@/lib/company/correlation-map'

/** Synthetic data for automated tests only; never used as a production fallback. */
export function company(id: number, name: string, overrides: Partial<CorrelationNode> = {}): CorrelationNode {
  return {
    id, name, name_reading: name, name_reading_missing: false, short_business: `${name}の事業説明`,
    short_business_known: true, main_business: `${name}が提供するサービス`, description: '',
    business_tags: [], business_tags_status: 'unavailable', market_type: '', market_type_known: false,
    is_listed: false, is_focus: id === 1, detail_path: `/company/${id}`, ...overrides,
  }
}
export function subsidiary(parent: number, child: number, overrides: Partial<CapitalEdge> = {}): CapitalEdge {
  return { id: parent * 100 + child, parent_id: parent, child_id: child, relation_type: 'capital_subsidiary', relation_label: '子会社', description: '', description_known: false, ...overrides }
}
export function sampleMap(): CorrelationMap {
  return {
    focus_company_id: 1,
    nodes: [company(1, '青葉テクノロジー'), company(2, '青葉ホールディングス'),
      company(3, 'あおばアプリ'), company(4, 'あおばクラウド'), company(5, 'あおばデータ'),
      company(6, 'あおばネット'), company(7, 'あおば保守'), company(8, 'あおば共同事業'),
      company(9, '東日本クラウド', { business_tags: ['クラウド'], business_tags_status: 'available' }),
      company(10, 'あおば取引先')],
    capital_edges: [subsidiary(2, 1), ...[3, 4, 5, 6, 7].map((id) => subsidiary(1, id)), subsidiary(4, 9), subsidiary(1, 8, { relation_type: 'capital_affiliate', relation_label: '関連会社' })],
    business_edges: [{ id: 1, from_id: 10, to_id: 1, direction_known: false, relation_type: 'business', relation_label: '業務委託', description: 'システムの保守を共同で行う。', description_known: true }],
    meta: { direct_parent_ids: [2], direct_child_ids: [3, 4, 5, 6, 7, 8], direct_child_total: 6, initial_child_limit: 4, initial_child_ids: [3, 4, 5, 6], remaining_child_count: 2, truncated: false, max_depth: 3, max_nodes: 100 },
  }
}
