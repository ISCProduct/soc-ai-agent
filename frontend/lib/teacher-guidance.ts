import { authService } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/config'

export interface StudentGuidance {
  id: number
  kind: string
  message: string
  suggested_industries?: string[]
  created_at: string
}

export async function fetchActiveGuidances(): Promise<StudentGuidance[]> {
  const res = await fetch(`${BACKEND_URL}/api/user/guidances`, {
    cache: 'no-store',
    headers: authService.getUserFetchHeaders(),
  })
  if (!res.ok) throw new Error('guidances')
  const data = (await res.json()) as { guidances?: StudentGuidance[] }
  return data.guidances ?? []
}

export async function dismissGuidance(id: number): Promise<void> {
  const res = await fetch(`${BACKEND_URL}/api/user/guidances/${id}/dismiss`, {
    method: 'POST',
    headers: authService.getUserFetchHeaders(),
  })
  if (!res.ok) throw new Error('dismiss-guidance')
}

export async function sendTeacherGuidance(
  studentId: number,
  body: { kind: string; message?: string; suggested_industries?: string[] },
): Promise<void> {
  const res = await fetch(`/api/admin/teacher/students/${studentId}/guidances`, {
    method: 'POST',
    headers: {
      ...authService.getAdminFetchHeaders(),
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({})) as { message?: string; error?: string }
    throw new Error(data.message || data.error || '案内の送信に失敗しました')
  }
}
