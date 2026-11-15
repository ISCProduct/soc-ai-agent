import type { MouseEvent } from 'react'
import type { Company } from '../../types'

/** 企業行が親から受け取る共通の状態・ハンドラ */
export interface CompanyRowContext {
  band: { min: number; max: number }
  maxAxisCount: number
  isProvisional: boolean
  isDesktop: boolean
  isGuestUser: boolean
  applyEnabled: boolean
  favoritingId: number | null
  applyingId: number | null
  onSelectCompany: (company: Company) => void
  onToggleFavorite: (e: MouseEvent, company: Company) => void
  onApply: (e: MouseEvent, company: Company) => void
  onNavigate: (path: string) => void
}
