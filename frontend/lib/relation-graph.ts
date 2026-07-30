import type { MarketType } from '@/lib/company-data'

// --- 編集リストのグルーピング（企業ごと・関連会社ごとにまとめる） -----------------

export type RelationEntry = {
  name: string
  relation_type: string
  ratio?: number
  description?: string
}

export type RelationCategory = 'capital' | 'business' | 'gbiz'

export const RELATION_CATEGORY_ORDER: RelationCategory[] = ['capital', 'business', 'gbiz']

export const RELATION_CATEGORY_LABELS: Record<RelationCategory, string> = {
  capital: '資本関係（子会社・関連会社）',
  business: '取引関係',
  gbiz: 'gBiz関係（調達・補助金）',
}

const RELATION_TYPE_TO_CATEGORY: Record<string, RelationCategory> = {
  capital_subsidiary: 'capital',
  capital_affiliate: 'capital',
  business_partner: 'business',
  business_procurement: 'gbiz',
  business_subsidy: 'gbiz',
}

export function categoryForRelationType(relationType: string): RelationCategory {
  return RELATION_TYPE_TO_CATEGORY[relationType] ?? 'business'
}

export type GroupedRelationEntry<T extends RelationEntry> = {
  /** 元の relations 配列でのインデックス（編集・削除ハンドラに渡す用） */
  index: number
  relation: T
}

export type CompanyRelationGroup<T extends RelationEntry> = {
  name: string
  entries: GroupedRelationEntry<T>[]
}

export type CategoryRelationGroup<T extends RelationEntry> = {
  category: RelationCategory
  companies: CompanyRelationGroup<T>[]
}

/**
 * 関連企業リストを「関係カテゴリ別（資本/取引/gBiz）」×「関連企業名ごと」にグルーピングする。
 * 同じ関連企業に複数の関係タイプが紐づく場合は1企業カードにまとめる。
 * 元配列の並び順・インデックスは保持し、編集/削除ハンドラがそのまま使えるようにする。
 */
export function groupRelationsByCategory<T extends RelationEntry>(relations: T[]): CategoryRelationGroup<T>[] {
  const byCategory = new Map<RelationCategory, Map<string, GroupedRelationEntry<T>[]>>()

  relations.forEach((relation, index) => {
    const category = categoryForRelationType(relation.relation_type)
    const companyKey = relation.name.trim() || '(未入力)'
    if (!byCategory.has(category)) byCategory.set(category, new Map())
    const companies = byCategory.get(category)!
    if (!companies.has(companyKey)) companies.set(companyKey, [])
    companies.get(companyKey)!.push({ index, relation })
  })

  return RELATION_CATEGORY_ORDER.filter((category) => byCategory.has(category)).map((category) => ({
    category,
    companies: Array.from(byCategory.get(category)!.entries()).map(([name, entries]) => ({ name, entries })),
  }))
}

// --- 多段階資本関係グラフのレイアウト（親会社→子会社→孫会社等） -----------------

export interface RelationGraphNode {
  id: number
  name: string
  market_type?: MarketType | ''
  is_listed: boolean
  is_focus: boolean
}

export interface RelationGraphCapitalEdge {
  parent_id: number
  child_id: number
  relation_type: string
  ratio?: number
}

export interface RelationGraphBusinessEntry {
  company_id: number
  name: string
  relation_type: string
  description?: string
}

export interface CompanyRelationGraph {
  company_id: number
  nodes: RelationGraphNode[]
  capital_edges: RelationGraphCapitalEdge[]
  /** API は未設定時に null を返すことがある */
  business_relations?: RelationGraphBusinessEntry[] | null
  truncated?: boolean
}

export interface CapitalGraphPosition {
  /** 起点企業からの世代差。0=起点企業、負値=親方向、正値=子方向。 */
  level: number
  /** 同じ level 内での並び順（0始まり）。 */
  column: number
  x: number
  y: number
}

const LEVEL_HEIGHT = 160
const COLUMN_WIDTH = 220

/**
 * 資本関係グラフのノードに、起点企業を基準とした世代(level)とX/Y座標を割り当てる。
 * 子会社方向は正の level、親会社方向は負の level になる（同じ親を持つ兄弟会社は同じ level）。
 * 循環関係があっても各ノードは最初に到達した時点の level で確定し無限ループしない。
 */
export function layoutCapitalGraph(graph: Pick<CompanyRelationGraph, 'company_id' | 'nodes' | 'capital_edges'>): Map<number, CapitalGraphPosition> {
  const neighborDeltas = new Map<number, Array<{ id: number; delta: number }>>()
  const addNeighbor = (from: number, to: number, delta: number) => {
    if (!neighborDeltas.has(from)) neighborDeltas.set(from, [])
    neighborDeltas.get(from)!.push({ id: to, delta })
  }
  for (const edge of graph.capital_edges ?? []) {
    // parent -> child は子方向なので +1、child -> parent は親方向なので -1
    addNeighbor(edge.parent_id, edge.child_id, 1)
    addNeighbor(edge.child_id, edge.parent_id, -1)
  }

  const levelById = new Map<number, number>()
  levelById.set(graph.company_id, 0)
  const queue: number[] = [graph.company_id]
  while (queue.length > 0) {
    const current = queue.shift()!
    const currentLevel = levelById.get(current)!
    for (const { id, delta } of neighborDeltas.get(current) ?? []) {
      if (levelById.has(id)) continue
      levelById.set(id, currentLevel + delta)
      queue.push(id)
    }
  }

  const nodesByLevel = new Map<number, number[]>()
  for (const node of graph.nodes ?? []) {
    const level = levelById.get(node.id) ?? 0
    if (!nodesByLevel.has(level)) nodesByLevel.set(level, [])
    nodesByLevel.get(level)!.push(node.id)
  }

  const positions = new Map<number, CapitalGraphPosition>()
  for (const [level, ids] of nodesByLevel.entries()) {
    ids.forEach((id, column) => {
      positions.set(id, {
        level,
        column,
        x: column * COLUMN_WIDTH,
        y: level * LEVEL_HEIGHT,
      })
    })
  }

  return positions
}
