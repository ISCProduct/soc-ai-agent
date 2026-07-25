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
}

/** recommendations API の空レスポンス時に付く診断情報 */
export interface RecommendationsDiagnostics {
  user_score_count?: number
  active_company_count?: number
  weight_profile_count?: number
}
