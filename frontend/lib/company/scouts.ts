import { companyAuthService } from '@/lib/company/auth'

export type ScoutStatus = 'sent' | 'viewed' | 'accepted' | 'declined'

export interface ScoutTemplate {
  id: number
  title: string
  body: string
  created_at: string
  updated_at: string
}

export interface CompanyScout {
  id: number
  user_id: number
  student_name?: string
  template_id?: number
  message: string
  status: ScoutStatus
  created_at: string
}

export interface ScoutCooldown {
  user_id: number
  remaining_ms: number
  blocked: boolean
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
    let message = 'リクエストに失敗しました'
    try {
      const body = (await res.json()) as { message?: string; error?: string; remaining_ms?: number }
      message = body.message ?? body.error ?? message
      if (res.status === 429 && body.remaining_ms != null) {
        const hours = Math.max(1, Math.ceil(body.remaining_ms / (60 * 60 * 1000)))
        message = `同じ学生への再送はあと約${hours}時間空ける必要があります`
      }
    } catch {
      // ignore
    }
    throw new Error(message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export function interpolateScoutBody(
  body: string,
  vars: { studentName: string; companyName: string },
): string {
  return body
    .replaceAll('{{学生名}}', vars.studentName)
    .replaceAll('{{企業名}}', vars.companyName)
    .replaceAll('{{name}}', vars.studentName)
    .replaceAll('{{company}}', vars.companyName)
}

export const companyScoutService = {
  listTemplates(): Promise<{ items: ScoutTemplate[] }> {
    return request('/scout-templates')
  },

  createTemplate(title: string, body: string): Promise<ScoutTemplate> {
    return request('/scout-templates', {
      method: 'POST',
      body: JSON.stringify({ title, body }),
    })
  },

  updateTemplate(id: number, title: string, body: string): Promise<ScoutTemplate> {
    return request(`/scout-templates/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ title, body }),
    })
  },

  deleteTemplate(id: number): Promise<void> {
    return request(`/scout-templates/${id}`, { method: 'DELETE' })
  },

  listScouts(): Promise<{ items: CompanyScout[]; total: number }> {
    return request('/scouts')
  },

  send(input: { userId: number; templateId: number; message?: string }): Promise<CompanyScout> {
    return request('/scouts', {
      method: 'POST',
      body: JSON.stringify({
        user_id: input.userId,
        template_id: input.templateId,
        message: input.message ?? '',
      }),
    })
  },

  cooldown(userId: number): Promise<ScoutCooldown> {
    return request(`/scouts/cooldown?user_id=${userId}`)
  },
}

export const SCOUT_STATUS_LABEL: Record<ScoutStatus, string> = {
  sent: '未読',
  viewed: '既読',
  accepted: '承諾',
  declined: '辞退',
}
