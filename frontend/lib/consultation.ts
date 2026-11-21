import { authService } from '@/lib/auth'

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
  /** 発言は保存できたが、スレッドの読み直しに失敗した（読み直す） */
  stale?: boolean
}

export type StudentConsultationItem = {
  guidance_id: number
  kind: string
  message: string
  created_at: string
  updated_at: string
  unread: number
}

export type ConsultationInboxItem = {
  guidance_id: number
  student_user_id: number
  student_name: string
  kind: string
  first_message: string
  unread: number
  last_message_at: string
  last_message_id: number
}

export const MAX_CONSULTATION_LENGTH = 1000

/**
 * 本文の文字数。Backend（Go の rune 数）と同じく、絵文字などのサロゲートペアも1文字と数える。
 * 送信前の前後の空白は Backend が取り除くので、ここでも除いて数える。
 */
export function countConsultationChars(value: string): number {
  return Array.from(value.trim()).length
}

async function readError(res: Response, fallback: string): Promise<never> {
  const data = (await res.json().catch(() => ({}))) as { message?: string; error?: string }
  throw new Error(data.message || data.error || fallback)
}

// ---- 学生（同一オリジンの /api/user/consultations を通す） ----

async function studentFetch(path: string, init?: RequestInit): Promise<Response> {
  await authService.ensureFreshUserToken()
  return fetch(`/api/user/consultations${path}`, {
    cache: 'no-store',
    ...init,
    headers: { ...authService.getUserFetchHeaders(), ...(init?.body ? { 'Content-Type': 'application/json' } : {}) },
  })
}

export async function fetchStudentConsultations(cursor?: string): Promise<{ items: StudentConsultationItem[]; next_cursor: string }> {
  const res = await studentFetch(cursor ? `?cursor=${encodeURIComponent(cursor)}` : '')
  if (!res.ok) return readError(res, '相談を読み込めませんでした')
  const data = (await res.json()) as { items?: StudentConsultationItem[]; next_cursor?: string }
  return { items: data.items ?? [], next_cursor: data.next_cursor ?? '' }
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

export async function fetchTeacherInbox(
  schoolId?: number,
  beforeId?: number,
): Promise<{ items: ConsultationInboxItem[]; next_before_id?: number }> {
  const params = new URLSearchParams()
  if (schoolId !== undefined) params.set('school_id', String(schoolId))
  if (beforeId) params.set('before_id', String(beforeId))
  const res = await fetch(`/api/admin/teacher/consultations${params.size ? `?${params}` : ''}`, {
    headers: authService.getAdminFetchHeaders(),
    cache: 'no-store',
  })
  if (!res.ok) return readError(res, '相談の一覧を読み込めませんでした')
  const data = (await res.json()) as { items?: ConsultationInboxItem[]; next_before_id?: number }
  return { items: data.items ?? [], next_before_id: data.next_before_id || undefined }
}

export async function fetchTeacherThread(id: number): Promise<ConsultationThread> {
  const res = await fetch(`/api/admin/teacher/consultations/${id}`, { headers: authService.getAdminFetchHeaders(), cache: 'no-store' })
  if (!res.ok) return readError(res, res.status === 404 ? '相談が見つかりません（担当校以外の学生の相談は開けません）' : '相談を読み込めませんでした')
  return (await res.json()) as ConsultationThread
}

/** 先生の返信。seenMessageId は、返信を書いた画面に出ていた最後の発言の ID（そこまでの学生の発言を既読にする）。 */
export async function replyAsTeacher(id: number, body: string, seenMessageId?: number): Promise<ConsultationThread> {
  const res = await fetch(`/api/admin/teacher/consultations/${id}/messages`, {
    method: 'POST',
    headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ body, seen_message_id: seenMessageId ?? 0 }),
  })
  if (!res.ok) return readError(res, '返信を送れませんでした')
  return (await res.json()) as ConsultationThread
}
