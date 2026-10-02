import { authService } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/config'

export type ScoutStatus = 'sent' | 'viewed' | 'accepted' | 'declined'

export interface StudentScout {
  id: number
  company_id: number
  company_name: string
  message: string
  status: ScoutStatus
  created_at: string
}

export interface StudentScoutList {
  items: StudentScout[]
  total: number
  blocked_company_ids: number[]
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  await authService.ensureFreshUserToken()
  const res = await fetch(`${BACKEND_URL}/api/user${path}`, {
    ...init,
    headers: {
      ...authService.getUserFetchHeaders(),
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...init?.headers,
    },
  })
  if (!res.ok) {
    let message = 'リクエストに失敗しました'
    try {
      const body = (await res.json()) as { message?: string; error?: string }
      message = body.message ?? body.error ?? message
    } catch {
      // ignore
    }
    throw new Error(message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const studentScoutService = {
  list(): Promise<StudentScoutList> {
    return request('/scouts')
  },

  view(id: number): Promise<StudentScout & { company_id: number }> {
    return request(`/scouts/${id}/view`, { method: 'POST' })
  },

  decline(id: number): Promise<{ id: number; status: ScoutStatus }> {
    return request(`/scouts/${id}/decline`, { method: 'POST' })
  },

  blockCompany(companyId: number): Promise<void> {
    return request('/scout-blocks', {
      method: 'POST',
      body: JSON.stringify({ company_id: companyId }),
    })
  },
}

export const SCOUT_STATUS_LABEL: Record<ScoutStatus, string> = {
  sent: '未読',
  viewed: '既読',
  accepted: '承諾',
  declined: '辞退',
}
