import {
  groupRelationsByCategory,
  layoutCapitalGraph,
  layoutCapitalGraphFromEdges,
  layoutBusinessGraph,
  groupIdsByConnectedComponent,
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

    it('assigns negative levels going up to the parent', () => {
      const positions = layoutCapitalGraph(baseGraph())
      expect(positions.get(4)?.level).toBe(-1)
    })

    it('places siblings sharing a parent at the same level as the focus company', () => {
      const graph = baseGraph()
      // 4(親) の別の子会社 5 を追加 -> 起点企業(1)の兄弟にあたるので level 0 になるはず
      graph.nodes.push({ id: 5, name: '兄弟会社', is_listed: false, is_focus: false })
      graph.capital_edges.push({ parent_id: 4, child_id: 5, relation_type: 'capital_subsidiary' })

      const positions = layoutCapitalGraph(graph)
      expect(positions.get(5)?.level).toBe(0)
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

    it('assigns distinct x within the same level and level-based y', () => {
      const graph = baseGraph()
      graph.nodes.push({ id: 5, name: '兄弟会社', is_listed: false, is_focus: false })
      graph.capital_edges.push({ parent_id: 4, child_id: 5, relation_type: 'capital_subsidiary' })

      const positions = layoutCapitalGraph(graph)
      const focusX = positions.get(1)!.x
      const siblingX = positions.get(5)!.x
      expect(focusX).not.toBe(siblingX)
      expect(positions.get(2)!.y).toBeGreaterThan(positions.get(1)!.y)
      expect(positions.get(4)!.y).toBeLessThan(positions.get(1)!.y)
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

  describe('groupIdsByConnectedComponent', () => {
    it('つながっている企業同士を隣接させる(登録順のばらばらな配置を避ける)', () => {
      // 1-2が資本関係、3-4が事業関係で繋がっているが、元の配列では1,3,2,4の順で登録されている
      const ordered = groupIdsByConnectedComponent(
        [1, 3, 2, 4],
        [
          { parent_id: 1, child_id: 2 },
          { from_id: 3, to_id: 4 },
        ],
      )
      // 1の直後に(元は離れていた)2が来て、同じ成分がまとまること
      expect(ordered.indexOf(2)).toBe(ordered.indexOf(1) + 1)
      expect(ordered.indexOf(4)).toBe(ordered.indexOf(3) + 1)
    })

    it('関係を持たない企業も結果に含める', () => {
      const ordered = groupIdsByConnectedComponent([1, 2], [])
      expect(ordered.sort()).toEqual([1, 2])
    })
  })
})
