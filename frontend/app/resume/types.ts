/**
 * 履歴書レビューページ向けの共有型。
 */

export type ReviewItem = {
  id: number
  page_number: number
  severity: string
  message: string
  suggestion?: string
}

export type ReviewResult = {
  review: {
    id: number
    /** 総合スコア(0-100)。採点できなかったときは null（#1529） */
    score: number | null
    /** 項目別スコア(0-5)のJSON文字列。内訳が無いレビューでは未設定 */
    item_scores_json?: string | null
    summary: string
  }
  items: ReviewItem[]
  annotated_available: boolean
}

export type CompanyCandidate = {
  name: string
  description?: string
  source: string
  exists?: boolean
  confidence?: string
  company_id?: number
  evidence_urls?: string[]
}

export type SeverityChipColor = 'error' | 'warning' | 'info' | 'default'

export type SeverityConfig = {
  color: SeverityChipColor
  label: string
  borderColor: string
}
