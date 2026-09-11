/**
 * 企業詳細ページ用のユーティリティ。
 */

export function parseJsonArray(s?: string): string[] {
  if (!s) return []
  try {
    const parsed: unknown = JSON.parse(s)
    return Array.isArray(parsed) ? parsed.map(String) : []
  } catch {
    return s.split(',').map((x) => x.trim()).filter(Boolean)
  }
}

import { formatEmployeeCount } from '@/lib/company-data'

/** API / プロキシ応答から企業オブジェクトを取り出す */
export function unwrapCompanyRecord(raw: unknown): Record<string, unknown> | null {
  if (!raw || typeof raw !== 'object') return null
  if (Array.isArray(raw)) return null
  const obj = raw as Record<string, unknown>
  if (obj.data && typeof obj.data === 'object' && !Array.isArray(obj.data)) {
    return obj.data as Record<string, unknown>
  }
  return obj
}

export type CompanyDetailViewModel = {
  id: number
  name: string
  industry: string
  location: string
  employees: string
  description: string
  matchScore: number
  tags: string[]
  tech_stack?: string
  techStack: string[]
  infra_stack?: string
  cicd_tools?: string
  development_style?: string
  projectTypes: string[]
  salary: string
  benefits: string[]
  culture: string[]
  founded: string
  website: string
  size: string
  parentCompany?: string
  subsidiaries?: string[]
  partnerships?: string[]
}

export function mapCompanyApiToViewModel(raw: unknown): CompanyDetailViewModel | null {
  const data = unwrapCompanyRecord(raw)
  if (!data) return null

  const id = typeof data.id === 'number' ? data.id : Number(data.id)
  if (!Number.isFinite(id)) return null

  const cultureRaw = data.culture
  const culture = Array.isArray(cultureRaw)
    ? cultureRaw.map(String)
    : typeof cultureRaw === 'string' && cultureRaw
      ? [cultureRaw]
      : []

  const employeeCount = typeof data.employee_count === 'number' ? data.employee_count : undefined
  const employeeBasis = typeof data.employee_count_basis === 'string' ? data.employee_count_basis : undefined
  const foundedYear = typeof data.founded_year === 'number' ? data.founded_year : undefined
  const employees = formatEmployeeCount(employeeCount, employeeBasis)

  return {
    id,
    name: typeof data.name === 'string' ? data.name : `企業 ${id}`,
    industry: typeof data.industry === 'string' ? data.industry : '',
    location: typeof data.location === 'string' ? data.location : '',
    description: typeof data.description === 'string' ? data.description : '',
    tech_stack: typeof data.tech_stack === 'string' ? data.tech_stack : undefined,
    infra_stack: typeof data.infra_stack === 'string' ? data.infra_stack : undefined,
    cicd_tools: typeof data.cicd_tools === 'string' ? data.cicd_tools : undefined,
    development_style: typeof data.development_style === 'string' ? data.development_style : undefined,
    techStack: parseJsonArray(typeof data.tech_stack === 'string' ? data.tech_stack : undefined),
    matchScore: typeof data.matchScore === 'number' ? data.matchScore : 0,
    tags: Array.isArray(data.tags) ? data.tags.map(String) : [],
    projectTypes: Array.isArray(data.projectTypes) ? data.projectTypes.map(String) : [],
    salary: typeof data.salary === 'string' ? data.salary : '',
    benefits: Array.isArray(data.benefits) ? data.benefits.map(String) : [],
    culture,
    founded: foundedYear ? `${foundedYear}年` : '',
    website: typeof data.website_url === 'string' ? data.website_url : '',
    employees,
    size: employees ? `${employees}規模` : '',
    parentCompany: typeof data.parentCompany === 'string' ? data.parentCompany : undefined,
    subsidiaries: Array.isArray(data.subsidiaries) ? data.subsidiaries.map(String) : undefined,
    partnerships: Array.isArray(data.partnerships) ? data.partnerships.map(String) : undefined,
  }
}
