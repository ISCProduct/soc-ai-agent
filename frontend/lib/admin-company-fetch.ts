/**
 * 企業の主3種（基本・技術・ビジネス関係）一括取得。
 * Backend 専用 API POST /api/admin/companies/:id/fetch-primary を呼ぶ。
 */

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

function stepLabel(step: FetchPrimaryStep | undefined, label: string): string | null {
  if (!step?.status) return null
  if (step.status === 'fetched') {
    return step.count != null ? `${label}取得(${step.count})` : `${label}取得`
  }
  if (step.status === 'skipped') {
    const reason =
      step.detail === 'ttl' || step.detail === 'ttl_fresh'
        ? '取得済み'
        : step.detail === 'budget'
          ? '予算超過'
          : step.detail === 'fetcher unavailable'
            ? '取得器なし'
            : step.detail || ''
    return reason ? `${label}スキップ(${reason})` : `${label}スキップ`
  }
  if (step.status === 'empty') return `${label}取得ゼロ`
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

export function formatFetchPrimarySummary(data: FetchPrimaryResponse): string {
  return [
    stepLabel(data.info_step, '基本'),
    stepLabel(data.tech_step, '技術'),
    stepLabel(data.relations_step, '関係'),
  ]
    .filter(Boolean)
    .join(' / ')
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
