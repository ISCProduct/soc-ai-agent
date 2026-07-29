/**
 * 企業の主3種（基本・技術・ビジネス関係）一括取得。
 * Backend 専用 API POST /api/admin/companies/:id/fetch-primary を呼ぶ。
 */

import { resolveIndustryFieldProfile } from '@/lib/admin-company-field-profile'

export type FetchPrimaryStep = {
  status?: string
  detail?: string
  count?: number
  skipped?: boolean
}

export type FetchPrimaryResponse = {
  ok?: boolean
  force?: boolean
  company_id?: number
  company_name?: string
  aspects?: string[]
  errors?: string[]
  info_step?: FetchPrimaryStep
  tech_step?: FetchPrimaryStep
  relations_step?: FetchPrimaryStep
  info?: Record<string, unknown>
  tech?: Record<string, unknown>
  relations?: Record<string, unknown>
  company?: Record<string, unknown>
  error?: string
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function toRecordOrUndefined(value: unknown): Record<string, unknown> | undefined {
  return isRecord(value) ? value : undefined
}

function toStepOrUndefined(value: unknown): FetchPrimaryStep | undefined {
  if (!isRecord(value)) return undefined
  return {
    status: typeof value.status === 'string' ? value.status : undefined,
    detail: typeof value.detail === 'string' ? value.detail : undefined,
    count: typeof value.count === 'number' ? value.count : undefined,
    skipped: typeof value.skipped === 'boolean' ? value.skipped : undefined,
  }
}

function parseFetchPrimaryResponse(value: unknown): FetchPrimaryResponse {
  if (!isRecord(value)) {
    return {}
  }
  return {
    ok: typeof value.ok === 'boolean' ? value.ok : undefined,
    force: typeof value.force === 'boolean' ? value.force : undefined,
    company_id: typeof value.company_id === 'number' ? value.company_id : undefined,
    company_name: typeof value.company_name === 'string' ? value.company_name : undefined,
    aspects: Array.isArray(value.aspects) ? value.aspects.filter((v): v is string => typeof v === 'string') : undefined,
    errors: Array.isArray(value.errors) ? value.errors.filter((v): v is string => typeof v === 'string') : undefined,
    info_step: toStepOrUndefined(value.info_step),
    tech_step: toStepOrUndefined(value.tech_step),
    relations_step: toStepOrUndefined(value.relations_step),
    info: toRecordOrUndefined(value.info),
    tech: toRecordOrUndefined(value.tech),
    relations: toRecordOrUndefined(value.relations),
    company: toRecordOrUndefined(value.company),
    error: typeof value.error === 'string' ? value.error : undefined,
  }
}

function industryFromResponse(data: FetchPrimaryResponse, fallbackIndustry?: string | null): string {
  const fromCompany = data.company?.industry
  if (typeof fromCompany === 'string' && fromCompany.trim()) return fromCompany
  return fallbackIndustry?.trim() || ''
}

function techLabelForIndustry(industry?: string | null): string {
  return resolveIndustryFieldProfile(industry).techAspectLabel
}

function skipReasonLabel(detail?: string): string {
  switch (detail) {
    case 'ttl':
    case 'ttl_fresh':
      return '取得済み'
    case 'budget':
      return '予算超過'
    case 'fetcher unavailable':
      return '取得器なし'
    case 'industry_not_applicable':
      return '業種対象外'
    case 'optional_empty':
      return '任意・未特定'
    case 'public_info_sparse':
      return '公開情報限定'
    case 'confirmed_unlisted':
      return '非上場・関係なし'
    default:
      return detail || ''
  }
}

function stepLabel(step: FetchPrimaryStep | undefined, label: string): string | null {
  if (!step?.status) return null
  if (step.status === 'fetched') {
    if (step.detail === 'confirmed_unlisted') {
      return `${label}確認済(非上場・関係なし)`
    }
    return step.count != null ? `${label}取得(${step.count})` : `${label}取得`
  }
  if (step.status === 'skipped') {
    const reason = skipReasonLabel(step.detail)
    return reason ? `${label}スキップ(${reason})` : `${label}スキップ`
  }
  if (step.status === 'empty') {
    if (step.detail === 'public_info_sparse') {
      return `${label}確認済(概要未特定)`
    }
    return `${label}取得ゼロ`
  }
  if (step.status === 'error') {
    const reason =
      step.detail === 'budget'
        ? '予算超過'
        : step.detail
          ? `(${step.detail})`
          : ''
    return `${label}失敗${reason}`
  }
  return `${label}${step.status}`
}

/** 業界制約を踏まえ、警告対象の empty ステップか。 */
export function isActionableEmptyStep(
  step: FetchPrimaryStep | undefined,
  aspect: 'info' | 'tech' | 'relations',
  industry?: string | null,
): boolean {
  if (step?.status !== 'empty') return false
  // 公開情報から概要が取れなかった確認済みは、手入力検討のソフト警告対象
  if (aspect === 'info' && step.detail === 'public_info_sparse') return true
  if (aspect === 'tech') {
    return resolveIndustryFieldProfile(industry).requireTechForPublish
  }
  return true
}

export function hasActionableSoftEmpty(
  data: FetchPrimaryResponse,
  fallbackIndustry?: string | null,
): boolean {
  const industry = industryFromResponse(data, fallbackIndustry)
  return (
    isActionableEmptyStep(data.info_step, 'info', industry) ||
    isActionableEmptyStep(data.tech_step, 'tech', industry) ||
    isActionableEmptyStep(data.relations_step, 'relations', industry)
  )
}

export function formatFetchPrimarySummary(
  data: FetchPrimaryResponse,
  fallbackIndustry?: string | null,
): string {
  const industry = industryFromResponse(data, fallbackIndustry)
  const techLabel = techLabelForIndustry(industry)
  return [
    stepLabel(data.info_step, '基本'),
    stepLabel(data.tech_step, techLabel),
    stepLabel(data.relations_step, '関係'),
  ]
    .filter(Boolean)
    .join(' / ')
}

/** empty になった項目だけを短く列挙（警告文用）。 */
export function formatFetchPrimaryEmptyAspects(
  data: FetchPrimaryResponse,
  fallbackIndustry?: string | null,
): string {
  const industry = industryFromResponse(data, fallbackIndustry)
  const techLabel = techLabelForIndustry(industry)
  const parts: string[] = []
  if (isActionableEmptyStep(data.info_step, 'info', industry)) {
    parts.push(data.info_step?.detail === 'public_info_sparse' ? '会社概要（公開情報から未特定）' : '会社概要')
  }
  if (isActionableEmptyStep(data.tech_step, 'tech', industry)) parts.push(techLabel)
  if (isActionableEmptyStep(data.relations_step, 'relations', industry)) parts.push('関連企業')
  return parts.join('・')
}

/** 主3種を1 API で取得する。 */
export async function fetchCompanyPrimary(
  companyId: string | number,
  headers: Record<string, string>,
  force = false,
): Promise<{ ok: boolean; status: number; data: FetchPrimaryResponse }> {
  const qs = force ? '?force=true' : ''
  const res = await fetch(`/api/admin/companies/${companyId}/fetch-primary${qs}`, {
    method: 'POST',
    headers,
  })
  const raw: unknown = await res.json().catch(() => ({}))
  const data = parseFetchPrimaryResponse(raw)
  return { ok: res.ok, status: res.status, data }
}
