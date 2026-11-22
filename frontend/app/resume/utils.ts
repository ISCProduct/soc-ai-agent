/**
 * 履歴書レビューページ向けの純粋ヘルパー（React state 非依存）。
 */
import type { CompanyCandidate, ReviewResult } from './types'

// 重要度の表示は形式チェック（#1530）と共通なので lib/severity.ts に置き、ここからも使えるようにする
export { getSeverityConfig, severityConfig } from '@/lib/severity'

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

export type FindingSeverity = 'critical' | 'warning' | 'info'

/** 画面に並べる指摘1件。AIの指摘（items）とルールの形式チェック（format_findings）を同じ形にそろえる。 */
export type Finding = {
  /** ページ順に振った通し番号（1始まり）。書類の強調と一覧で同じ番号を使う */
  no: number
  page: number
  severity: FindingSeverity
  source: 'ai' | 'rule'
  message: string
  /** 指摘が指している原文 */
  quote: string
  /** AIの書き換え案。読み取りが不確かなときは空 */
  rewrite: string
  /** 読み取りが不確かで、書き換え案を出していない */
  withheld: boolean
  /** 書き換え案と別に伝える補足 */
  supplement: string
  /** 書類の上の位置。無いときは強調を重ねられない（形式チェックなど） */
  box: BBox | null
}

export const FINDING_SEVERITY_LABEL: Record<FindingSeverity, string> = {
  critical: '重大',
  warning: '注意',
  info: '情報',
}

function normalizeSeverity(severity: string): FindingSeverity {
  return severity === 'critical' || severity === 'info' ? severity : 'warning'
}

/**
 * 結果を、ページ順（同じページではAI→ルールの順）に並べて番号を振る。
 * 書類側の番号付き強調と、指摘の一覧で同じ番号を指すため。
 */
export function buildFindings(review: Pick<ReviewResult, 'items' | 'format_findings'>): Finding[] {
  const ai = (review.items ?? []).map((item) => {
    const withheld = item.rewrite_withheld === 'low_ocr_confidence'
    const rewrite = withheld ? '' : (item.rewrite?.trim() ?? '')
    const suggestion = item.suggestion?.trim() ?? ''
    return {
      page: item.page_number > 0 ? item.page_number : 1,
      severity: normalizeSeverity(item.severity),
      source: 'ai' as const,
      message: item.message?.trim() ?? '',
      quote: item.quote?.trim() ?? '',
      rewrite,
      withheld,
      // 書き換え案と同じ文は繰り返さない
      supplement: suggestion !== '' && suggestion !== rewrite ? suggestion : '',
      box: parseBBox(item.bbox),
    }
  })
  const rule = (review.format_findings ?? []).map((f) => ({
    page: f.page_hint && f.page_hint > 0 ? f.page_hint : 1,
    severity: normalizeSeverity(f.severity),
    source: 'rule' as const,
    message: f.message?.trim() ?? '',
    quote: f.excerpt?.trim() ?? '',
    rewrite: '',
    withheld: false,
    supplement: f.fix?.trim() ?? '',
    box: null,
  }))
  const sourceRank = { ai: 0, rule: 1 }
  return [...ai, ...rule]
    .map((f, i) => ({ f, i }))
    .sort((a, b) => a.f.page - b.f.page || sourceRank[a.f.source] - sourceRank[b.f.source] || a.i - b.i)
    .map(({ f }, idx) => ({ ...f, no: idx + 1 }))
}

export function countBySeverity(findings: Finding[]): Record<FindingSeverity, number> {
  const counts: Record<FindingSeverity, number> = { critical: 0, warning: 0, info: 0 }
  for (const f of findings) counts[f.severity]++
  return counts
}

/** 指摘があるページの最大値。指摘が無ければ1ページとみなす */
export function pageCountOf(findings: Finding[]): number {
  return Math.max(1, ...findings.map((f) => f.page))
}

/** 書類の上の位置（画素。左上が原点）と、そのときのページの大きさ。比にすると、描く大きさによらず同じ場所を指せる。 */
export type BBox = { x0: number; y0: number; x1: number; y1: number; pageWidth: number; pageHeight: number }

/**
 * 指摘の bbox（JSON文字列）を読む。
 * Backend は {"bbox":[x0,y0,x1,y1],"page_width":W,"page_height":H} で保存する。
 * ページの大きさが無い古い形式や、壊れた値は null にして、強調を重ねない（間違った場所を指さない）。
 */
export function parseBBox(raw: string | null | undefined): BBox | null {
  if (!raw) return null
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return null
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null
  const { bbox, page_width: pageWidth, page_height: pageHeight } = parsed as { bbox?: unknown; page_width?: unknown; page_height?: unknown }
  if (!Array.isArray(bbox) || bbox.length < 4) return null
  const [x0, y0, x1, y1] = bbox
  const nums = [x0, y0, x1, y1, pageWidth, pageHeight]
  if (!nums.every((n) => typeof n === 'number' && Number.isFinite(n))) return null
  if ((pageWidth as number) <= 0 || (pageHeight as number) <= 0 || (x1 as number) <= (x0 as number) || (y1 as number) <= (y0 as number)) return null
  return { x0: x0 as number, y0: y0 as number, x1: x1 as number, y1: y1 as number, pageWidth: pageWidth as number, pageHeight: pageHeight as number }
}

/** ページの大きさに対する割合（%）。ページの外にはみ出す分は切る。 */
export function overlayRect(box: BBox): { left: number; top: number; width: number; height: number } {
  const clamp = (v: number) => Math.min(100, Math.max(0, v))
  const left = clamp((box.x0 / box.pageWidth) * 100)
  const top = clamp((box.y0 / box.pageHeight) * 100)
  const right = clamp((box.x1 / box.pageWidth) * 100)
  const bottom = clamp((box.y1 / box.pageHeight) * 100)
  return { left, top, width: right - left, height: bottom - top }
}
