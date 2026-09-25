/**
 * マッチング結果ページ向けの共有型。
 */

export interface CategoryScores {
  technical: number
  teamwork: number
  leadership: number
  creativity: number
  stability: number
  growth: number
  work_life: number
  challenge: number
  detail: number
  communication: number
}

export interface Company {
  id: string
  matchId?: number
  name: string
  industry: string
  location: string
  employees: string
  description: string
  matchScore: number
  /** matchScore の算出に使えた軸の数（0-10、#1124）。少ないほど根拠が薄い */
  matchedAxisCount?: number
  tags: string[]
  techStack: string[]
  categoryScores?: CategoryScores
  isFavorited?: boolean
  isApplied?: boolean
  applicationId?: number
}

export interface AnalysisScores {
  job: number
  interest: number
  aptitude: number
  future: number
}

export interface SuggestedRole {
  title: string
  reason: string
}

export interface SnackbarState {
  open: boolean
  message: string
  severity: 'success' | 'error'
  /** snackbar内に導線ボタンを出す場合の遷移先（例: 応募後の選考管理画面） */
  actionHref?: string
  actionLabel?: string
}

/** recommendations API の空レスポンス時に付く診断情報 */
export interface RecommendationsDiagnostics {
  user_score_count?: number
  active_company_count?: number
  weight_profile_count?: number
  /** 公開中なのに重視度プロファイルを持たない企業数（#1380。マッチング対象外） */
  companies_without_profile?: number
}
