export interface PlacementBucket {
  label: string
  count: number
  share_percent: number
}

export interface PlacementSummary {
  recorded_count: number
  offer_rate_available: boolean
  note: string
  by_year: PlacementBucket[]
  by_industry: PlacementBucket[]
  by_school: PlacementBucket[]
}

function csvCell(value: string): string {
  if (/[",\n]/.test(value)) return `"${value.replaceAll('"', '""')}"`
  return value
}

/** 氏名を含まない内訳の CSV。割合は登録件数に対するもの。 */
export function placementCsv(summary: PlacementSummary): string {
  const lines = ['区分,ラベル,件数,登録内の割合']
  const push = (kind: string, rows: PlacementBucket[]) => {
    for (const row of rows) {
      lines.push([kind, csvCell(row.label), String(row.count), `${row.share_percent}%`].join(','))
    }
  }
  push('年度', summary.by_year ?? [])
  push('業界', summary.by_industry ?? [])
  push('学校', summary.by_school ?? [])
  return `\uFEFF${lines.join('\n')}\n`
}
