import { authService } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/config'

/** 教員と学生の相談（案内への返信・学生からの質問、#2028）。 */
export type ConsultationMessage = {
  id: number
  guidance_id: number
  author_role: 'student' | 'teacher'
  body: string
  read_at: string | null
  created_at: string
}

export type ConsultationThread = {
  guidance_id: number
  student_user_id: number
  kind: string
  message: string
  created_at: string
  messages: ConsultationMessage[]
}

export type ConsultationInboxItem = {
  guidance_id: number
  student_user_id: number
  student_name: string
  kind: string
  first_message: string
  unread: number
  last_message_at: string
}

export const MAX_CONSULTATION_LENGTH = 1000

async function readError(res: Response, fallback: string): Promise<never> {
  const data = (await res.json().catch(() => ({}))) as { message?: string; error?: string }
  throw new Error(data.message || data.error || fallback)
}

// ---- 学生（既存の案内と同じく Backend を直接呼ぶ） ----

async function studentFetch(path: string, init?: RequestInit): Promise<Response> {
  await authService.ensureFreshUserToken()
  return fetch(`${BACKEND_URL}/api/user/consultations${path}`, {
    cache: 'no-store',
    ...init,
    headers: { ...authService.getUserFetchHeaders(), ...(init?.body ? { 'Content-Type': 'application/json' } : {}) },
  })
}

export async function askTeacher(body: string): Promise<ConsultationThread> {
  const res = await studentFetch('', { method: 'POST', body: JSON.stringify({ body }) })
  if (!res.ok) return readError(res, '質問を送れませんでした')
  return (await res.json()) as ConsultationThread
}

export async function fetchStudentThread(id: number): Promise<ConsultationThread> {
  const res = await studentFetch(`/${id}`)
  if (!res.ok) return readError(res, '相談を読み込めませんでした')
  return (await res.json()) as ConsultationThread
}

export async function replyAsStudent(id: number, body: string): Promise<ConsultationThread> {
  const res = await studentFetch(`/${id}/messages`, { method: 'POST', body: JSON.stringify({ body }) })
  if (!res.ok) return readError(res, '返信を送れませんでした')
  return (await res.json()) as ConsultationThread
}

export async function fetchStudentUnread(): Promise<{ by_guidance: Record<string, number>; total: number }> {
  const res = await studentFetch('/unread')
  if (!res.ok) return readError(res, '未読を読み込めませんでした')
  return (await res.json()) as { by_guidance: Record<string, number>; total: number }
}

// ---- 先生（管理画面の中継ルートを通す） ----

export async function fetchTeacherInbox(schoolId?: number): Promise<ConsultationInboxItem[]> {
  const qs = schoolId !== undefined ? `?school_id=${schoolId}` : ''
  const res = await fetch(`/api/admin/teacher/consultations${qs}`, { headers: authService.getAdminFetchHeaders(), cache: 'no-store' })
  if (!res.ok) return readError(res, '相談の一覧を読み込めませんでした')
  const data = (await res.json()) as { items?: ConsultationInboxItem[] }
  return data.items ?? []
}

export async function fetchTeacherThread(id: number): Promise<ConsultationThread> {
  const res = await fetch(`/api/admin/teacher/consultations/${id}`, { headers: authService.getAdminFetchHeaders(), cache: 'no-store' })
  if (!res.ok) return readError(res, res.status === 403 ? 'この学生の相談を見る権限がありません' : '相談を読み込めませんでした')
  return (await res.json()) as ConsultationThread
}

export async function replyAsTeacher(id: number, body: string): Promise<ConsultationThread> {
  const res = await fetch(`/api/admin/teacher/consultations/${id}/messages`, {
    method: 'POST',
    headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ body }),
  })
  if (!res.ok) return readError(res, '返信を送れませんでした')
  return (await res.json()) as ConsultationThread
}
