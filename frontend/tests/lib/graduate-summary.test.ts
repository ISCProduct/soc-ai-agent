import { placementCsv, type PlacementSummary } from '@/lib/admin/graduate-summary'

const summary: PlacementSummary = {
  recorded_count: 2,
  offer_rate_available: false,
  note: '内定率は計算していません',
  by_year: [{ label: '2024卒', count: 2, share_percent: 100 }],
  by_industry: [{ label: 'A,B', count: 2, share_percent: 100 }],
  by_school: [],
}

describe('placementCsv', () => {
  it('氏名を出さず、カンマを含むラベルは引用する', () => {
    const csv = placementCsv(summary)
    expect(csv.startsWith('\uFEFF区分,ラベル,件数,登録内の割合\n')).toBe(true)
    expect(csv).toContain('年度,2024卒,2,100%')
    expect(csv).toContain('"A,B"')
    expect(csv).not.toContain('氏名')
  })
})
