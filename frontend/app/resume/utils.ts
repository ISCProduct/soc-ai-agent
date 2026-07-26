/**
 * 履歴書レビューページ向けの純粋ヘルパー（React state 非依存）。
 */
import type { CompanyCandidate, SeverityConfig } from './types'

/** 指摘事項の重要度表示設定 */
export const severityConfig: Record<string, Omit<SeverityConfig, 'color'> & { color: 'error' | 'warning' | 'info' }> = {
  critical: { color: 'error', label: '重大', borderColor: '#d32f2f' },
  warning: { color: 'warning', label: '注意', borderColor: '#ed6c02' },
  info: { color: 'info', label: '情報', borderColor: '#0288d1' },
}

/** 未知の severity にはデフォルト表示を返す */
export function getSeverityConfig(severity: string): SeverityConfig {
  return severityConfig[severity] ?? { color: 'default', label: severity, borderColor: '#9e9e9e' }
}

/** API エラーテキストからユーザー向けメッセージを抽出する */
export function parseApiErrorMessage(errText: string, defaultMessage: string): string {
  if (!errText) return defaultMessage
  try {
    const parsed = JSON.parse(errText) as { error?: string; message?: string }
    return parsed?.error || parsed?.message || errText
  } catch {
    return errText || defaultMessage
  }
}

/** DB 企業検索 API のレスポンスを候補一覧に変換する */
export function mapDbCompanyResults(data: unknown): CompanyCandidate[] {
  const payload = data as { companies?: unknown } | unknown[]
  const companies = (Array.isArray(payload)
    ? payload
    : (payload as { companies?: unknown }).companies || payload || []) as {
    id?: number
    name?: string
    description?: string
  }[]

  return (Array.isArray(companies) ? companies : [])
    .filter((c) => c?.name)
    .map((c) => ({
      name: c.name as string,
      description: c.description || '',
      source: 'db',
      exists: true,
      confidence: 'high',
      company_id: c.id,
    }))
}

/** WEB 企業検索 API のレスポンスを候補一覧に変換する */
export function mapWebSearchResults(data: unknown): CompanyCandidate[] {
  const results = (data as { results?: unknown }).results as {
    name?: string
    description?: string
    source?: string
    exists?: boolean
    confidence?: string
    company_id?: number
    evidence_urls?: string[]
  }[] | undefined

  return (Array.isArray(results) ? results : [])
    .filter((c) => c?.name && c.exists !== false)
    .map((c) => ({
      name: c.name as string,
      description: c.description || '',
      source: c.source || 'web_search',
      exists: true,
      confidence: c.confidence,
      company_id: c.company_id,
      evidence_urls: c.evidence_urls || [],
    }))
}

/**
 * 注釈 PDF レスポンスかどうかをヘッダーとステータスから判定する。
 * Content-Type が application/octet-stream でも Range 成功なら実体ありとみなす。
 */
export function isAnnotatedPdfResponse(
  contentType: string,
  contentDisposition: string,
  status: number,
): boolean {
  const normalizedType = contentType.toLowerCase()
  if (normalizedType.includes('application/pdf')) return true

  const normalizedDisposition = contentDisposition.toLowerCase()
  if (normalizedDisposition.includes('.pdf')) return true

  return status === 206 || status === 200
}
