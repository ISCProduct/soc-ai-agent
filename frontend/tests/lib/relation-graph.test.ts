import {
  groupRelationsByCategory,
  layoutCapitalGraph,
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
})
