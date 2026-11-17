import { authService } from '@/lib/auth'

/**
 * 卒業生の進路実績（#2032）。自分の学校の先輩の就職先を、人数で見る（氏名などは出ない）。
 * 3人未満の区分は人数を出さない（学内で誰か特定できるため）。total が null なら「少数」、
 * by_year_few / by_position_few は、表示していない少数の年度・職種がほかにあることを表す。
 */
export type YearCount = { year: number; count: number }

export type GraduateOutcomeCompanySummary = {
  company_id: number
  company_name: string
  industry: string
  total: number | null
  by_year: YearCount[]
  by_year_few: boolean
}

export type GraduateOutcomeList = {
  school_linked: boolean
  total: number
  companies: GraduateOutcomeCompanySummary[]
  years: number[]
  industries: string[]
}

export type GraduateOutcomeCompany = {
  school_linked: boolean
  total: number | null
  by_year: YearCount[]
  by_year_few: boolean
  by_position: { title: string; count: number }[]
  by_position_few: boolean
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
