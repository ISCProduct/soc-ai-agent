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

const COLUMN_WIDTH = 220
/** 同一levelの兄弟ノードをこの数を超えたら複数行に折り返す。子会社が多い企業でも
 * 1行に並べてfitViewが極端に縮小されテキストが潰れる事態を避けるため（#970フォローアップ）。 */
const MAX_NODES_PER_ROW = 6
/** 折り返した行同士の縦間隔。levelをまたぐ間隔(LEVEL_GAP)より詰めて、
 * 「同じ世代の折り返し」と「世代の切り替わり」を視覚的に区別できるようにする。 */
const WRAPPED_ROW_HEIGHT = 130
/** 世代(level)が切り替わる際に追加する縦の余白。 */
const LEVEL_GAP = 60

/**
 * 起点ノードからのBFSで各ノードにlevel(世代/距離)を割り当て、X/Y座標を確定する汎用ロジック。
 * #970: 相関図の各コンポーネントが個別に持っていた円形/登録順配置を、関係性に基づく配置へ統一する。
 * さらに、同じlevelの兄弟ノードが多い場合（例: 子会社が数十社ある企業）は複数行に折り返し、
 * 1行に並べてfitViewが極端に縮小される問題を避ける（#970フォローアップ）。
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

  // level の昇順（親方向の負値→起点0→子方向の正値）に、折り返しで使った行数分だけ
  // 縦位置を積み上げていく。折り返しが発生したlevelは複数行分の高さを消費するため、
  // 固定間隔ではなく累積オフセットで次levelの開始Y座標を決める。
  const sortedLevels = Array.from(nodesByLevel.keys()).sort((a, b) => a - b)
  const positions = new Map<number, CapitalGraphPosition>()
  let y = 0
  for (const level of sortedLevels) {
    const ids = nodesByLevel.get(level)!
    const rowCount = Math.max(1, Math.ceil(ids.length / MAX_NODES_PER_ROW))

    ids.forEach((id, index) => {
      const rowIndex = Math.floor(index / MAX_NODES_PER_ROW)
      const rowStart = rowIndex * MAX_NODES_PER_ROW
      const nodesInThisRow = Math.min(MAX_NODES_PER_ROW, ids.length - rowStart)
      const indexInRow = index - rowStart
      // 各行は起点(x=0)を中心に左右対称になるよう配置し、行ごとの企業数が違っても
      // 上位levelのノードと視覚的に揃って見えるようにする。
      const rowWidth = (nodesInThisRow - 1) * COLUMN_WIDTH
      positions.set(id, {
        level,
        column: index,
        x: indexInRow * COLUMN_WIDTH - rowWidth / 2,
        y: y + rowIndex * WRAPPED_ROW_HEIGHT,
      })
    })

    y += rowCount * WRAPPED_ROW_HEIGHT + LEVEL_GAP
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

/** 資本/事業関係の隣接リストを構築する内部ヘルパー。 */
function buildNeighborMap(edges: Array<CapitalEdgeLike & BusinessEdgeLike>): Map<number, number[]> {
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
  return neighbors
}

export interface RelationCluster {
  /** クラスタ内で最も接続数(次数)が多い企業ID。起点企業表示への遷移先として使う。 */
  hubId: number
  /** クラスタに含まれる全企業ID（探索順）。 */
  memberIds: number[]
}

/**
 * 単一の起点企業を指定しない「全体表示」向け。企業ID一覧を資本/事業関係の連結成分
 * (connected component)ごとにまとめ、各成分の代表企業(次数最大=ハブ企業)を求める。
 * 成分はメンバー数の多い順にソートする。
 *
 * #970フォローアップ: 数百社をまとめて1枚のグラフに描画すると、ハブ企業(メガバンク等)が
 * 複数クラスタにまたがる関係を持つため、レイアウトを工夫してもエッジが画面を横断してしまい
 * 本質的に読みにくい。「全部を1枚に描く」のをやめ、クラスタ単位のカード一覧として提示し、
 * カードを選ぶとそのクラスタの代表企業を起点にした（すでに読みやすい）単一企業表示に遷移する
 * 設計に変更した。この関数はそのカード一覧のデータを作る。
 */
export function computeRelationClusters(
  ids: number[],
  edges: Array<CapitalEdgeLike & BusinessEdgeLike>,
): RelationCluster[] {
  const neighbors = buildNeighborMap(edges)
  const idSet = new Set(ids)
  const visited = new Set<number>()
  const clusters: RelationCluster[] = []

  for (const start of ids) {
    if (visited.has(start)) continue
    const memberIds: number[] = []
    const queue = [start]
    visited.add(start)
    while (queue.length > 0) {
      const current = queue.shift()!
      memberIds.push(current)
      for (const neighborId of neighbors.get(current) ?? []) {
        if (visited.has(neighborId) || !idSet.has(neighborId)) continue
        visited.add(neighborId)
        queue.push(neighborId)
      }
    }
    const hubId = memberIds.reduce((best, id) =>
      (neighbors.get(id)?.length ?? 0) > (neighbors.get(best)?.length ?? 0) ? id : best,
    memberIds[0])
    clusters.push({ hubId, memberIds })
  }

  return clusters.sort((a, b) => b.memberIds.length - a.memberIds.length)
}
