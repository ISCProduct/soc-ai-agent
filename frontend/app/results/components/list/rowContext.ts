import type { MouseEvent } from 'react'
import type { Company, MatchFeedbackVerdict } from '../../types'
import type { AxisBreakdown } from '@/lib/axis-match'

/** 企業行が親から受け取る共通の状態・ハンドラ */
export interface CompanyRowContext {
  isDesktop: boolean
  maxAxisCount: number
  isProvisional: boolean
  isGuestUser: boolean
  applyEnabled: boolean
  favoritingId: number | null
  applyingId: number | null
  /** 「合う／合わない」を送信中の推薦。送信中は押せない */
  feedbackingId: number | null
  onSelectCompany: (company: Company) => void
  onToggleFavorite: (e: MouseEvent, company: Company) => void
  /** 推薦への「合う／合わない」。省略すると、ボタンを出さない */
  onFeedback?: (e: MouseEvent, company: Company, verdict: MatchFeedbackVerdict) => void
  onApply: (e: MouseEvent, company: Company) => void
  onNavigate: (path: string) => void
  /** 軸ごとの内訳を開いている推薦（match_id）（SOCAIAGENT-611） */
  openAxes: ReadonlySet<number>
  /** 取得済みの内訳。閉じて開き直すたびに取り直さない */
  axesCache: Map<number, AxisBreakdown>
  onToggleAxes: (matchId: number) => void
}
