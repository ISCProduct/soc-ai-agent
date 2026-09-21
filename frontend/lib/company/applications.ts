import { companyAuthService } from '@/lib/company/auth'

// 企業ポータルのダッシュボードと応募者管理（#1320）。
//
// company_id はサーバー側がJWTから解決するため、クライアントからは送らない。
// 送っても無視される。

export interface CompanyDashboard {
  pending_applications: number
  published_jobs: number
  new_candidates: number
  new_candidate_window_days: number
}

export interface ApplicationListItem {
  id: number
  user_id: number
  // student_name はスカウト公開に同意した学生のみ入る。
  // 未同意なら空文字。#1319 で方針が決まるまでは既存の同意ルールに従う。
  student_name: string
  status: string
  notes: string
  applied_at?: string
  status_updated_at?: string
  created_at: string
}

export interface ApplicationListResult {
  applications: ApplicationListItem[]
  total: number
  limit: number
  offset: number
}

// 選考ステータスの表示名。値は Backend の ValidStatuses と揃えること
// （docs/requirements/application-status-transition.md）。
export const APPLICATION_STATUS_LABELS: Record<string, string> = {
  not_applied: '未応募',
  applied: '応募済み',
  document_screening: '書類選考中',
  document_passed: '書類通過',
  interview_scheduled: '面接予定',
  interview_in_progress: '面接中',
  offered: '内定',
  accepted: '内定承諾',
  withdrawn: '辞退',
  rejected: '不採用',
}

// 企業側が次の行動を取るべきステータス。
// Backend の PortalPendingStatuses と揃える。
export const PENDING_STATUSES = [
  'applied',
  'document_screening',
  'document_passed',
  'interview_scheduled',
  'interview_in_progress',
  'offered',
] as const

// 企業が実行できる遷移。Backend の adminAllowedTransitions のうち、
// 企業側の操作としてUIに出すもの。ここに無い遷移を送ると 409 が返る。
export const ALLOWED_TRANSITIONS: Record<string, string[]> = {
  applied: ['document_screening', 'rejected'],
  document_screening: ['document_passed', 'rejected'],
  document_passed: ['interview_scheduled'],
  interview_scheduled: ['interview_in_progress'],
  interview_in_progress: ['offered', 'rejected'],
  offered: [],
}

export function statusLabel(status: string): string {
  return APPLICATION_STATUS_LABELS[status] ?? status
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  await companyAuthService.ensureFreshToken()
  const res = await fetch(`/api/company-portal${path}`, {
    ...init,
    headers: {
      ...companyAuthService.getAuthHeaders(),
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...init?.headers,
    },
  })
  if (!res.ok) {
    if (res.status === 403) {
      throw new Error('この操作を行う権限がありません。')
    }
    if (res.status === 409) {
      // 遷移表に無い変更。画面の選択肢が古い可能性がある。
      throw new Error('この選考ステータスへは変更できません。画面を再読み込みしてください。')
    }
    throw new Error('リクエストに失敗しました')
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const companyApplicationService = {
  async fetchDashboard(): Promise<CompanyDashboard> {
    return request<CompanyDashboard>('/dashboard')
  },

  async list(params: { status?: string; limit?: number; offset?: number } = {}): Promise<ApplicationListResult> {
    const q = new URLSearchParams()
    if (params.status) q.set('status', params.status)
    if (params.limit !== undefined) q.set('limit', String(params.limit))
    if (params.offset !== undefined) q.set('offset', String(params.offset))
    const suffix = q.toString() ? `?${q.toString()}` : ''
    return request<ApplicationListResult>(`/applications${suffix}`)
  },

  async updateStatus(applicationId: number, status: string, notes?: string): Promise<ApplicationListItem> {
    return request<ApplicationListItem>(`/applications/${applicationId}/status`, {
      method: 'PATCH',
      body: JSON.stringify({ status, ...(notes !== undefined ? { notes } : {}) }),
    })
  },
}
