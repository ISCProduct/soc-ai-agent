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
 * 起点ノードからのBFSで各ノードにlevel(世代/距離)を割り当て、同じlevel内では列(column)で
 * 横に並べてX/Y座標を確定する汎用ロジック。#970: 相関図の各コンポーネントが個別に持っていた
 * 円形/登録順配置を、関係性に基づく配置へ統一するための共通処理。
 * neighborsOf は、あるノードIDから見た隣接ノードと世代差分(delta)の一覧を返す関数。
 * 有向関係（資本関係の親→子など）はdeltaで向きを表し、無向関係（取引関係など）は常に+1にする。
 * 起点から辿り着けないノード（別の連結成分）はlevel 0として扱う。
 * 循環関係があっても各ノードは最初に到達した時点のlevelで確定し無限ループしない。
 */
function layoutFromFocus(
  focusId: number,
  allNodeIds: number[],
  neighborsOf: (id: number) => Array<{ id: number; delta: number }>,
): Map<number, CapitalGraphPosition> {
  const levelById = new Map<number, number>()
  levelById.set(focusId, 0)
  const queue: number[] = [focusId]
  while (queue.length > 0) {
    const current = queue.shift()!
    const currentLevel = levelById.get(current)!
    for (const { id, delta } of neighborsOf(current)) {
      if (levelById.has(id)) continue
      levelById.set(id, currentLevel + delta)
      queue.push(id)
    }
  }

  const nodesByLevel = new Map<number, number[]>()
  for (const id of allNodeIds) {
    const level = levelById.get(id) ?? 0
    if (!nodesByLevel.has(level)) nodesByLevel.set(level, [])
    nodesByLevel.get(level)!.push(id)
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

/**
 * 資本関係グラフのノードに、起点企業を基準とした世代(level)とX/Y座標を割り当てる。
 * 子会社方向は正の level、親会社方向は負の level になる（同じ親を持つ兄弟会社は同じ level）。
 */
export function layoutCapitalGraph(graph: {
  company_id: number
  nodes: Pick<RelationGraphNode, 'id'>[]
  capital_edges: Pick<RelationGraphCapitalEdge, 'parent_id' | 'child_id'>[]
}): Map<number, CapitalGraphPosition> {
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

  return layoutFromFocus(
    graph.company_id,
    (graph.nodes ?? []).map((n) => n.id),
    (id) => neighborDeltas.get(id) ?? [],
  )
}

/** parent_id/child_id を持つ資本関係の最小形（フロント各所の CapitalRelation 等から作れる）。 */
export interface CapitalEdgeLike {
  parent_id?: number
  child_id?: number
}

/**
 * layoutCapitalGraph の簡易版。CompanyRelationGraph 形式に変換せず、起点企業ID・関連企業ID一覧・
 * 資本関係の配列（parent_id/child_id を含むもの）から直接レイアウトを計算する。
 */
export function layoutCapitalGraphFromEdges(
  focusId: number,
  nodeIds: number[],
  edges: CapitalEdgeLike[],
): Map<number, CapitalGraphPosition> {
  return layoutCapitalGraph({
    company_id: focusId,
    nodes: nodeIds.map((id) => ({ id, name: '', is_listed: false, is_focus: id === focusId })),
    capital_edges: edges.filter(
      (e): e is { parent_id: number; child_id: number } => !!e.parent_id && !!e.child_id,
    ),
  })
}

/** from_id/to_id を持つ取引関係などの最小形。 */
export interface BusinessEdgeLike {
  from_id?: number
  to_id?: number
}

/**
 * 取引関係など「向きに世代の意味がない」関係を、起点企業からのBFS距離でlevel分けして配置する。
 * 直接関係する企業ほど起点に近いlevelにまとまるため、無関係な登録順で配置がばらつく問題を避けられる。
 */
export function layoutBusinessGraph(
  focusId: number,
  nodeIds: number[],
  edges: BusinessEdgeLike[],
): Map<number, CapitalGraphPosition> {
  const neighbors = new Map<number, number[]>()
  const addNeighbor = (a: number, b: number) => {
    if (!neighbors.has(a)) neighbors.set(a, [])
    neighbors.get(a)!.push(b)
  }
  for (const e of edges) {
    if (!e.from_id || !e.to_id) continue
    addNeighbor(e.from_id, e.to_id)
    addNeighbor(e.to_id, e.from_id)
  }

  return layoutFromFocus(focusId, nodeIds, (id) =>
    (neighbors.get(id) ?? []).map((neighborId) => ({ id: neighborId, delta: 1 })),
  )
}

/**
 * 単一の起点企業がない「全体表示」向け。企業ID一覧を、資本/事業関係で繋がっている者同士が
 * 連番で隣り合うよう連結成分(connected component)ごとに並べ替える。グリッド配置と組み合わせることで、
 * 無関係な登録順による配置のばらつきを避け、関連企業が視覚的に近くまとまるようにする。
 * 元のids配列内での最初の登場順は、成分の出現順・成分内の探索順として維持する。
 */
export function groupIdsByConnectedComponent(
  ids: number[],
  edges: Array<CapitalEdgeLike & BusinessEdgeLike>,
): number[] {
  const neighbors = new Map<number, number[]>()
  const addNeighbor = (a: number, b: number) => {
    if (!neighbors.has(a)) neighbors.set(a, [])
    neighbors.get(a)!.push(b)
  }
  for (const e of edges) {
    const a = e.parent_id ?? e.from_id
    const b = e.child_id ?? e.to_id
    if (!a || !b) continue
    addNeighbor(a, b)
    addNeighbor(b, a)
  }

  const idSet = new Set(ids)
  const visited = new Set<number>()
  const result: number[] = []
  for (const start of ids) {
    if (visited.has(start)) continue
    const queue = [start]
    visited.add(start)
    while (queue.length > 0) {
      const current = queue.shift()!
      result.push(current)
      for (const neighborId of neighbors.get(current) ?? []) {
        if (visited.has(neighborId) || !idSet.has(neighborId)) continue
        visited.add(neighborId)
        queue.push(neighborId)
      }
    }
  }
  return result
}
