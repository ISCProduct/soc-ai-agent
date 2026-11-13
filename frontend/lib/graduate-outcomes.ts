import { authService } from '@/lib/auth'

/** 卒業生の進路実績（#2032）。自分の学校の先輩の就職先を、人数で見る（氏名などは出ない）。 */
export type YearCount = { year: number; count: number }

export type GraduateOutcomeList = {
  school_linked: boolean
  total: number
  companies: { company_id: number; company_name: string; industry: string; total: number; by_year: YearCount[] }[]
  years: number[]
  industries: string[]
}

export type GraduateOutcomeCompany = {
  school_linked: boolean
  total: number
  by_year: YearCount[]
  by_position: { title: string; count: number }[]
}

export async function fetchGraduateOutcomes(filter: { year?: number; industry?: string }): Promise<GraduateOutcomeList> {
  const params = new URLSearchParams()
  if (filter.year) params.set('year', String(filter.year))
  if (filter.industry) params.set('industry', filter.industry)
  const res = await fetch(`/api/graduate-outcomes${params.size ? `?${params}` : ''}`, { headers: authService.getUserFetchHeaders() })
  if (!res.ok) throw new Error(`graduate-outcomes ${res.status}`)
  return (await res.json()) as GraduateOutcomeList
}

export async function fetchCompanyGraduateOutcome(companyId: string | number): Promise<GraduateOutcomeCompany> {
  const res = await fetch(`/api/graduate-outcomes/companies/${encodeURIComponent(String(companyId))}`, { headers: authService.getUserFetchHeaders() })
  if (!res.ok) throw new Error(`graduate-outcome ${res.status}`)
  return (await res.json()) as GraduateOutcomeCompany
}
