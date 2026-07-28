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

function stepLabel(step: FetchPrimaryStep | undefined, label: string): string | null {
  if (!step?.status) return null
  if (step.status === 'fetched') {
    return step.count != null ? `${label}取得(${step.count})` : `${label}取得`
  }
  if (step.status === 'skipped') return `${label}スキップ`
  if (step.status === 'empty') return `${label}取得ゼロ`
  if (step.status === 'error') return `${label}失敗`
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
  const data = (await res.json().catch(() => ({}))) as FetchPrimaryResponse
  return { ok: res.ok, status: res.status, data }
}
