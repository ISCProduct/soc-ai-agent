import { authService } from '@/lib/auth'

/**
 * 卒業生の進路実績（#2032）。自分の学校の先輩の就職先を、人数で見る（氏名などは出ない）。
 * 3人未満の区分（企業 × 年度 × 職種）は人数を出さない（学内で誰か特定できるため）。total・年度別・職種別は
 * どれも見せる区分の和で、隠した人数は入らない。total が null なら見せる区分が無い（「少数」）。
 * total_few / by_year_few / by_position_few は、数に入れていない少数の卒業生がほかにいることを表す（「＋少数」）。
 */
export type YearCount = { year: number; count: number }

export type GraduateOutcomeCompanySummary = {
  company_id: number
  company_name: string
  industry: string
  total: number | null
  total_few: boolean
  by_year: YearCount[]
  by_year_few: boolean
}

export type GraduateOutcomeList = {
  school_linked: boolean
  total: number
  total_few: boolean
  companies: GraduateOutcomeCompanySummary[]
  years: number[]
  industries: string[]
}

export type GraduateOutcomeCompany = {
  school_linked: boolean
  total: number | null
  total_few: boolean
  by_year: YearCount[]
  by_year_few: boolean
  by_position: { title: string; count: number }[]
  by_position_few: boolean
}

/** 見せる区分の和に、数に入れていない少数がいれば「＋少数」を付ける。total が null なら「少数」。 */
export function totalLabel(total: number | null, few: boolean): string {
  if (total == null) return '少数'
  return few ? `${total}人＋少数` : `${total}人`
}

/** 年度が未登録（0）の実績は「年度未登録」と出す。 */
export function yearLabel(year: number): string {
  return year > 0 ? `${year}年卒` : '年度未登録'
}

export async function fetchGraduateOutcomes(
  filter: { year?: number; industry?: string },
  signal?: AbortSignal,
): Promise<GraduateOutcomeList> {
  const params = new URLSearchParams()
  if (filter.year) params.set('year', String(filter.year))
  if (filter.industry) params.set('industry', filter.industry)
  const res = await fetch(`/api/graduate-outcomes${params.size ? `?${params}` : ''}`, { headers: authService.getUserFetchHeaders(), signal })
  if (!res.ok) throw new Error(`graduate-outcomes ${res.status}`)
  return (await res.json()) as GraduateOutcomeList
}

/** 1社の実績。企業が見つからない（404）なら null。 */
export async function fetchCompanyGraduateOutcome(companyId: string | number, signal?: AbortSignal): Promise<GraduateOutcomeCompany | null> {
  const res = await fetch(`/api/graduate-outcomes/companies/${encodeURIComponent(String(companyId))}`, { headers: authService.getUserFetchHeaders(), signal })
  if (res.status === 404) return null
  if (!res.ok) throw new Error(`graduate-outcome ${res.status}`)
  return (await res.json()) as GraduateOutcomeCompany
}
