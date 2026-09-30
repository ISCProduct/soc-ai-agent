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

/**
 * ルーブリックの評価項目（#1529）。順序・キーはバックエンドの
 * Backend/internal/services/resume/resume_rubric.go と対応させる。
 * ここに無いキーは表示しない（項目を増やしたら両方に足す）。
 */
export const RUBRIC_CRITERIA: { key: string; label: string }[] = [
  { key: 'specificity', label: '具体性' },
  { key: 'achievement', label: '成果の明示' },
  { key: 'role_fit', label: '職種適合' },
  { key: 'completeness', label: '記載の網羅性' },
  { key: 'readability', label: '読みやすさ' },
]

/** 項目スコアの満点（0〜5） */
export const RUBRIC_SCORE_MAX = 5

/**
 * item_scores_json を内訳表示用に変換する。
 * 壊れている・項目が欠けている・値域外の項目は落とす（誤った内訳を出さない）。
 * 1件も残らなければ空配列を返し、画面は「内訳なし」に倒す。
 */
export function parseItemScores(json?: string | null): { key: string; label: string; score: number }[] {
  if (!json) return []
  let parsed: unknown
  try {
    parsed = JSON.parse(json)
  } catch {
    return []
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return []

  const scores = parsed as Record<string, unknown>
  return RUBRIC_CRITERIA.flatMap(({ key, label }) => {
    const value = scores[key]
    if (typeof value !== 'number' || !Number.isFinite(value)) return []
    if (value < 0 || value > RUBRIC_SCORE_MAX) return []
    return [{ key, label, score: value }]
  })
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
  if (!data || typeof data !== 'object') return []

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
  if (!data || typeof data !== 'object') return []

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
 * status 200 のみの場合は content-length / content-range の存在を要求する。
 */
export function isAnnotatedPdfResponse(
  contentType: string,
  contentDisposition: string,
  status: number,
  contentLength?: string | null,
  contentRange?: string | null,
): boolean {
  const normalizedType = contentType.toLowerCase()
  if (normalizedType.includes('application/pdf')) return true

  const normalizedDisposition = contentDisposition.toLowerCase()
  if (normalizedDisposition.includes('.pdf')) return true

  if (status === 206) return true
  if (status === 200) {
    return Boolean(contentLength || contentRange)
  }
  return false
}

/**
 * 企業候補の取得元を学生向けの日本語に直す。
 * そのまま出すと `db` / `web_search` という内部の値が画面に出てしまう。
 */
export function companySourceLabel(source?: string): string {
  if (!source) return ''
  return source === 'db' ? '掲載企業' : 'Web'
}
