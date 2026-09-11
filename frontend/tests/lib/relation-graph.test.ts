import {
  groupRelationsByCategory,
  layoutCapitalGraph,
  layoutCapitalGraphFromEdges,
  layoutBusinessGraph,
  computeRelationClusters,
  type RelationEntry,
  type CompanyRelationGraph,
} from '@/lib/relation-graph'

describe('relation-graph', () => {
  describe('groupRelationsByCategory', () => {
    it('groups entries by category and merges same-named companies', () => {
      const relations: RelationEntry[] = [
        { name: 'A社', relation_type: 'capital_subsidiary' },
        { name: 'B社', relation_type: 'business_partner' },
        { name: 'A社', relation_type: 'capital_affiliate' },
        { name: 'C社', relation_type: 'business_procurement' },
      ]

      const grouped = groupRelationsByCategory(relations)

      expect(grouped.map((g) => g.category)).toEqual(['capital', 'business', 'gbiz'])

      const capital = grouped.find((g) => g.category === 'capital')!
      expect(capital.companies).toHaveLength(1)
      expect(capital.companies[0].name).toBe('A社')
      expect(capital.companies[0].entries.map((e) => e.index)).toEqual([0, 2])

      const business = grouped.find((g) => g.category === 'business')!
      expect(business.companies[0].name).toBe('B社')

      const gbiz = grouped.find((g) => g.category === 'gbiz')!
      expect(gbiz.companies[0].name).toBe('C社')
    })

    it('treats unknown relation types as business category and blank names as a single group', () => {
      const relations: RelationEntry[] = [
        { name: '', relation_type: 'something_else' },
        { name: '  ', relation_type: 'something_else' },
      ]

      const grouped = groupRelationsByCategory(relations)
      expect(grouped).toHaveLength(1)
      expect(grouped[0].category).toBe('business')
      expect(grouped[0].companies).toHaveLength(1)
      expect(grouped[0].companies[0].entries).toHaveLength(2)
    })

    it('returns an empty array for no relations', () => {
      expect(groupRelationsByCategory([])).toEqual([])
    })
  })

  describe('layoutCapitalGraph', () => {
    const baseGraph = (): Pick<CompanyRelationGraph, 'company_id' | 'nodes' | 'capital_edges'> => ({
      company_id: 1,
      nodes: [
        { id: 1, name: '起点企業', is_listed: false, is_focus: true },
        { id: 2, name: '子会社', is_listed: false, is_focus: false },
        { id: 3, name: '孫会社', is_listed: false, is_focus: false },
        { id: 4, name: '親会社', is_listed: false, is_focus: false },
      ],
      capital_edges: [
        { parent_id: 1, child_id: 2, relation_type: 'capital_subsidiary' },
        { parent_id: 2, child_id: 3, relation_type: 'capital_affiliate' },
        { parent_id: 4, child_id: 1, relation_type: 'capital_subsidiary' },
      ],
    })

    it('assigns level 0 to the focus company', () => {
      const positions = layoutCapitalGraph(baseGraph())
      expect(positions.get(1)?.level).toBe(0)
    })

    it('assigns positive levels going down to children/grandchildren', () => {
      const positions = layoutCapitalGraph(baseGraph())
      expect(positions.get(2)?.level).toBe(1)
      expect(positions.get(3)?.level).toBe(2)
    })

    // #1022: 起点企業が子会社であっても親会社が常に上に固定されないよう、
    // 資本関係の向き(parent/child)に関わらず起点からのホップ数のみでlevelを決める。
    it('assigns positive levels going up to the parent too (direction is not reflected)', () => {
      const positions = layoutCapitalGraph(baseGraph())
      expect(positions.get(4)?.level).toBe(1)
    })

    it('places siblings sharing a parent one hop away from the focus company', () => {
      const graph = baseGraph()
      // 4(親) の別の子会社 5 を追加 -> 起点企業(1)から見て2ホップ(1→4→5)なのでlevel 2になるはず
      graph.nodes.push({ id: 5, name: '兄弟会社', is_listed: false, is_focus: false })
      graph.capital_edges.push({ parent_id: 4, child_id: 5, relation_type: 'capital_subsidiary' })

      const positions = layoutCapitalGraph(graph)
      expect(positions.get(5)?.level).toBe(2)
    })

    it('does not loop forever when the edges contain a cycle', () => {
      const graph: Pick<CompanyRelationGraph, 'company_id' | 'nodes' | 'capital_edges'> = {
        company_id: 1,
        nodes: [
          { id: 1, name: 'A', is_listed: false, is_focus: true },
          { id: 2, name: 'B', is_listed: false, is_focus: false },
        ],
        capital_edges: [
          { parent_id: 1, child_id: 2, relation_type: 'capital_affiliate' },
          { parent_id: 2, child_id: 1, relation_type: 'capital_affiliate' },
        ],
      }
      const positions = layoutCapitalGraph(graph)
      expect(positions.size).toBe(2)
      expect(positions.get(1)?.level).toBe(0)
    })

    it('assigns distinct x within the same level and level-based y, with the focus company always on top', () => {
      const graph = baseGraph()
      // 2(子会社) の別ノード6を追加 -> 起点(1)から見て同じlevel1になるはず
      graph.nodes.push({ id: 6, name: '別の子会社', is_listed: false, is_focus: false })
      graph.capital_edges.push({ parent_id: 1, child_id: 6, relation_type: 'capital_subsidiary' })

      const positions = layoutCapitalGraph(graph)
      const childX = positions.get(2)!.x
      const otherChildX = positions.get(6)!.x
      expect(childX).not.toBe(otherChildX)
      // 親会社(4)も子会社(2)も起点(1)より1ホップなので、どちらも起点より下に配置される
      expect(positions.get(2)!.y).toBeGreaterThan(positions.get(1)!.y)
      expect(positions.get(4)!.y).toBeGreaterThan(positions.get(1)!.y)
    })
  })

  // #970: 登録順の円形/グリッド配置ではなく、関係性(BFS距離)ベースの配置になることを検証する。
  describe('layoutCapitalGraphFromEdges', () => {
    it('CapitalRelation形式の配列から直接levelを算出できる', () => {
      const positions = layoutCapitalGraphFromEdges(1, [1, 2, 3], [
        { parent_id: 1, child_id: 2 },
        { parent_id: 2, child_id: 3 },
      ])
      expect(positions.get(1)?.level).toBe(0)
      expect(positions.get(2)?.level).toBe(1)
      expect(positions.get(3)?.level).toBe(2)
    })

    it('資本関係にない企業(relation_typeがbusiness等)は無視して0除算(NaN角度)を起こさない', () => {
      const positions = layoutCapitalGraphFromEdges(1, [1], [])
      expect(positions.get(1)).toEqual({ level: 0, column: 0, x: 0, y: 0 })
    })

    // #1022: 子会社を起点にしても親会社が上に固定表示されないことを保証する回帰テスト。
    it('起点企業が子会社側でも、親会社は起点より下(level 1)に配置される', () => {
      const positions = layoutCapitalGraphFromEdges(2, [1, 2], [{ parent_id: 1, child_id: 2 }])
      expect(positions.get(2)?.level).toBe(0)
      expect(positions.get(1)?.level).toBe(1)
    })
  })

  describe('layoutBusinessGraph', () => {
    it('起点企業から直接つながる企業をlevel1、2ホップ先をlevel2に配置する', () => {
      const positions = layoutBusinessGraph(1, [1, 2, 3], [
        { from_id: 1, to_id: 2 },
        { from_id: 2, to_id: 3 },
      ])
      expect(positions.get(1)?.level).toBe(0)
      expect(positions.get(2)?.level).toBe(1)
      expect(positions.get(3)?.level).toBe(2)
    })

    it('to_id起点で登録されていても双方向に隣接とみなす', () => {
      const positions = layoutBusinessGraph(2, [1, 2], [{ from_id: 1, to_id: 2 }])
      expect(positions.get(1)?.level).toBe(1)
    })

    it('起点から辿れない企業(別の連結成分)はlevel0として扱いnodeIdsに含まれる全企業の座標を返す', () => {
      const positions = layoutBusinessGraph(1, [1, 2, 99], [{ from_id: 1, to_id: 2 }])
      expect(positions.size).toBe(3)
      expect(positions.get(99)?.level).toBe(0)
    })
  })

  // #970フォローアップ: 子会社が多い企業でも1行に並べず複数行に折り返すことで、
  // fitViewで極端に縮小されてテキストが潰れる「醜い」表示を避ける。
  describe('layoutCapitalGraphFromEdges (折り返しレイアウト)', () => {
    it('兄弟ノードが少ない場合は1行のまま(既存の見た目を壊さない)', () => {
      const positions = layoutCapitalGraphFromEdges(
        1,
        [1, 2, 3, 4],
        [1, 2, 3, 4].slice(1).map((childId) => ({ parent_id: 1, child_id: childId })),
      )
      const ys = [2, 3, 4].map((id) => positions.get(id)!.y)
      expect(new Set(ys).size).toBe(1) // 全員同じ行(同じy)
    })

    it('兄弟ノードが多い場合は複数行に折り返し、同じ行に収まらない', () => {
      const childIds = Array.from({ length: 37 }, (_, i) => i + 2)
      const positions = layoutCapitalGraphFromEdges(
        1,
        [1, ...childIds],
        childIds.map((childId) => ({ parent_id: 1, child_id: childId })),
      )
      const ys = new Set(childIds.map((id) => positions.get(id)!.y))
      // 37社を1行(6社まで)に収めることはできないので、複数行に分かれること
      expect(ys.size).toBeGreaterThan(1)
      // 各行の最大幅が無制限に伸び続けないこと(1行あたりの最大x幅が一定以下)
      const xs = childIds.map((id) => positions.get(id)!.x)
      const xRange = Math.max(...xs) - Math.min(...xs)
      expect(xRange).toBeLessThan(2000)
    })

    it('折り返しが発生しても親→子のlevel順（y座標の大小）は保たれる', () => {
      const childIds = Array.from({ length: 20 }, (_, i) => i + 2)
      const positions = layoutCapitalGraphFromEdges(
        1,
        [1, ...childIds],
        childIds.map((childId) => ({ parent_id: 1, child_id: childId })),
      )
      const focusY = positions.get(1)!.y
      for (const id of childIds) {
        expect(positions.get(id)!.y).toBeGreaterThan(focusY)
      }
    })

    it('折り返した各行は起点を中心に左右対称に配置される', () => {
      const childIds = Array.from({ length: 8 }, (_, i) => i + 2) // 6+2 -> 2行
      const positions = layoutCapitalGraphFromEdges(
        1,
        [1, ...childIds],
        childIds.map((childId) => ({ parent_id: 1, child_id: childId })),
      )
      const secondRowXs = childIds.slice(6).map((id) => positions.get(id)!.x)
      const center = secondRowXs.reduce((a, b) => a + b, 0) / secondRowXs.length
      expect(Math.abs(center)).toBeLessThan(1)
    })
  })

  describe('computeRelationClusters', () => {
    it('つながっている企業同士を同じクラスタにまとめる', () => {
      // 1-2が資本関係、3-4が事業関係で繋がっているが、元の配列では1,3,2,4の順で登録されている
      const clusters = computeRelationClusters(
        [1, 3, 2, 4],
        [
          { parent_id: 1, child_id: 2 },
          { from_id: 3, to_id: 4 },
        ],
      )
      expect(clusters).toHaveLength(2)
      expect(clusters.map((c) => c.memberIds.slice().sort((a, b) => a - b))).toEqual(
        expect.arrayContaining([[1, 2], [3, 4]]),
      )
    })

    it('関係を持たない企業もサイズ1のクラスタとして含める', () => {
      const clusters = computeRelationClusters([1, 2], [])
      expect(clusters).toHaveLength(2)
      expect(clusters.every((c) => c.memberIds.length === 1)).toBe(true)
    })

    it('クラスタをメンバー数の多い順にソートする', () => {
      const clusters = computeRelationClusters(
        [1, 2, 10, 11, 12, 13],
        [
          { parent_id: 1, child_id: 2 }, // クラスタA: 2社
          { parent_id: 10, child_id: 11 },
          { parent_id: 10, child_id: 12 },
          { parent_id: 10, child_id: 13 }, // クラスタB: 4社
        ],
      )
      expect(clusters[0].memberIds).toHaveLength(4)
      expect(clusters[1].memberIds).toHaveLength(2)
    })

    it('クラスタの代表企業(hubId)は次数(接続数)が最大のノードになる', () => {
      const clusters = computeRelationClusters(
        [10, 11, 12, 13],
        [
          { parent_id: 10, child_id: 11 },
          { parent_id: 10, child_id: 12 },
          { parent_id: 10, child_id: 13 },
        ],
      )
      expect(clusters[0].hubId).toBe(10) // 10は3社と接続、他は1社のみ
    })
  })
})
