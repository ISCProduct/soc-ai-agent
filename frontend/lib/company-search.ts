import { authService } from '@/lib/auth'

/**
 * 学生向けの企業検索（SOCAIAGENT-612）。業界・地域（都道府県）・職種・名前で絞り込む。
 * 並びは企業名順で、おすすめ順ではない（推薦や課金で上位に固定しない）。
 */

export const COMPANY_SEARCH_PAGE_SIZE = 20

export type CompanySearchFilter = {
  name?: string
  industry?: string
  prefecture?: string
  jobCategoryId?: number
  page: number
}

export type CompanySearchItem = {
  id: number
  name: string
  industry: string
  location: string
  description: string
  logo_url: string
  /** 出どころ（詳細画面と同じ判定で「AI推定」を出す） */
  source_type?: string
  source_url?: string
  last_fetch_confidence?: string
  last_model_used?: string
}

export type CompanySearchResult = { companies: CompanySearchItem[]; total: number }

export type CompanySearchOptions = {
  industries: string[]
  prefectures: string[]
  job_categories: { id: number; name: string }[]
}

/** 推薦にある企業のお気に入りの状態。推薦に無い企業は含まれない */
export type CompanyFavoriteState = { company_id: number; match_id: number; is_favorited: boolean }

export function buildSearchParams(f: CompanySearchFilter): URLSearchParams {
  const p = new URLSearchParams()
  if (f.name?.trim()) p.set('name', f.name.trim())
  if (f.industry) p.set('industry', f.industry)
  if (f.prefecture) p.set('prefecture', f.prefecture)
  if (f.jobCategoryId) p.set('job_category_id', String(f.jobCategoryId))
  p.set('limit', String(COMPANY_SEARCH_PAGE_SIZE))
  p.set('offset', String(Math.max(0, f.page - 1) * COMPANY_SEARCH_PAGE_SIZE))
  return p
}

export async function searchCompanies(f: CompanySearchFilter, signal?: AbortSignal): Promise<CompanySearchResult> {
  const res = await fetch(`/api/companies/search?${buildSearchParams(f)}`, { cache: 'no-store', signal })
  if (!res.ok) throw new Error(`search ${res.status}`)
  const data = (await res.json()) as Partial<CompanySearchResult>
  return { companies: data.companies ?? [], total: data.total ?? 0 }
}

export async function fetchSearchOptions(): Promise<CompanySearchOptions> {
  const res = await fetch('/api/companies/search-options', { cache: 'no-store' })
  if (!res.ok) throw new Error(`options ${res.status}`)
  const data = (await res.json()) as Partial<CompanySearchOptions>
  return { industries: data.industries ?? [], prefectures: data.prefectures ?? [], job_categories: data.job_categories ?? [] }
}

export async function fetchFavoriteStates(companyIds: number[]): Promise<CompanyFavoriteState[]> {
  if (companyIds.length === 0) return []
  const res = await fetch(`/api/companies/search/favorites?company_ids=${companyIds.join(',')}`, {
    headers: authService.getUserFetchHeaders(),
    cache: 'no-store',
  })
  if (!res.ok) throw new Error(`favorites ${res.status}`)
  const data = (await res.json()) as { favorites?: CompanyFavoriteState[] }
  return data.favorites ?? []
}

/** お気に入りの切り替え（推薦の match_id に付く。結果画面と同じ API） */
export async function toggleFavorite(matchId: number): Promise<void> {
  const res = await fetch('/api/chat/favorite', {
    method: 'POST',
    headers: { ...authService.getUserFetchHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ match_id: matchId }),
  })
  if (!res.ok) throw new Error(`favorite ${res.status}`)
}
