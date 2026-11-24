// 管理画面の企業一覧で使う型・定数・純関数。
import type { CompanyAspect } from '@/components/admin/CompanyAspectTabs'
import { resolveIndustryFieldProfile } from '@/lib/admin/company-field-profile'

export const PAGE_SIZE = 50
/** 企業間並列。I/O待ちなので 8 まで。それ以上は OpenAI RPM が先に壊れる。 */
export const GLOBAL_BATCH_CONCURRENCY = 8
/** 1リクエスト内でワーカープールを回す。6社区切りだと早い社が次を待てない。 */
export const GLOBAL_BATCH_WAVE_LIMIT = 24
export const GLOBAL_BATCH_PREVIEW_LIMIT = 50
export const GLOBAL_BATCH_MAX_ROUNDS = 20
export const WARM_L1_WAVE_LIMIT = 30
/** Backend の業界未設定フィルタ用センチネル */
export const INDUSTRY_UNSET = '__unset__'

export type Company = {
  id: number
  name: string
  industry?: string
  location?: string
  source_type?: string
  is_provisional?: boolean
  data_status?: string
  is_verified?: boolean
  info_fetched_at?: string | null
  jobs_fetched_at?: string | null
  tech_fetched_at?: string | null
  relations_fetched_at?: string | null
  website_url?: string
  description?: string
  tech_stack?: string
}

export type FilterStatus = 'all' | 'draft' | 'published'
export type FilterReadiness = 'all' | 'ready' | 'missing'

export type L1Coverage = {
  published_total: number
  info_fresh: number
  has_profile: number
  needs_warm: number
  info_rate: number
  profile_rate: number
  info_target?: number
  profile_target?: number
  below_target?: boolean
  alerts?: string[]
}

export type AspectKey = 'info' | 'tech' | 'relations' | 'jobs'

export const ASPECT_LABEL: Record<AspectKey, string> = {
  info: '会社概要',
  tech: '技術情報',
  relations: '関連企業',
  jobs: '求人',
}

export const ASPECT_PAGE: Partial<Record<AspectKey, CompanyAspect>> = {
  info: 'info',
  tech: 'tech',
  relations: 'relations',
}

export function missingAspects(c: Company): AspectKey[] {
  const profile = resolveIndustryFieldProfile(c.industry)
  const missing: AspectKey[] = []
  if (!c.info_fetched_at || !c.description || !c.website_url) missing.push('info')
  const tech = (c.tech_stack ?? '').trim()
  if (
    profile.requireTechForPublish &&
    (!c.tech_fetched_at || !tech || tech === '[]' || tech === 'null' || tech === '{}')
  ) {
    missing.push('tech')
  }
  if (!c.relations_fetched_at) missing.push('relations')
  if (!c.jobs_fetched_at) missing.push('jobs')
  return missing
}

/** 公開前かつ主情報がそろっている企業だけ一括公開の対象にする。 */
export function canSelectForPublish(c: Company): boolean {
  if (c.data_status === 'published') return false
  return missingAspects(c).filter((k) => k !== 'jobs').length === 0
}

export function aspectLabel(key: AspectKey, industry?: string): string {
  if (key === 'tech') {
    return resolveIndustryFieldProfile(industry).techAspectLabel
  }
  return ASPECT_LABEL[key]
}

export function subtitleLine(c: Company): string {
  const parts = [c.industry, c.location].filter((v) => Boolean(v && v.trim()))
  return parts.join('  ·  ')
}

export function statusLabel(status?: string): { label: string; color: 'warning' | 'success' | 'error' | 'default' } {
  if (status === 'published') return { label: '学生に公開中', color: 'success' }
  if (status === 'rejected') return { label: '非公開', color: 'error' }
  return { label: '公開前', color: 'warning' }
}

export function pct(rate: number) {
  return `${Math.round((rate || 0) * 100)}%`
}

export function industryGroupLabel(industry?: string): string {
  const trimmed = industry?.trim()
  return trimmed ? trimmed : '業界未設定'
}

export function groupCompaniesByIndustry(companies: Company[]): { key: string; label: string; items: Company[] }[] {
  const groups: { key: string; label: string; items: Company[] }[] = []
  const indexByKey = new Map<string, number>()
  for (const company of companies) {
    const key = company.industry?.trim() || INDUSTRY_UNSET
    const existing = indexByKey.get(key)
    if (existing === undefined) {
      indexByKey.set(key, groups.length)
      groups.push({ key, label: industryGroupLabel(company.industry), items: [company] })
    } else {
      groups[existing].items.push(company)
    }
  }
  return groups
}

/** 通信断など、応答を受け取れなかったときの文言（#2573） */
export const networkErrorMessage = (action: string) =>
  `${action}中に通信エラーが発生しました。時間をおいて再度お試しください。`
