import { authService } from '@/lib/auth'

export type AssessmentStatus = 'not_started' | 'in_progress' | 'submitted'

export interface AssessmentSummary {
  template_id: number
  title: string
  description: string
  question_count: number
  status: AssessmentStatus
  session_id?: string
  answered_count: number
  submitted_at?: string
  /** standard=運営標準 / school=学校独自（マッチングには使われない） */
  scope: AssessmentScope
  /** 受検期限（ISO）。無ければ期限なし */
  due_at?: string
  overdue: boolean
}

export type AssessmentScope = 'standard' | 'school'

export interface AssessmentQuestion {
  id: number
  position: number
  body: string
}

export interface AssessmentSession {
  session_id: string
  title: string
  status: AssessmentStatus
  questions: AssessmentQuestion[]
  /** question_id -> 1〜5（JSON のキーなので文字列） */
  answers: Record<string, number>
}

export interface AssessmentResult {
  session_id: string
  title: string
  submitted_at?: string
  scores: { category: string; score: number }[]
  scope: AssessmentScope
}

export const QUESTIONS_PER_PAGE = 2

export const LIKERT_OPTIONS = [
  { value: 1, label: 'あてはまらない' },
  { value: 2, label: 'あまりあてはまらない' },
  { value: 3, label: 'どちらともいえない' },
  { value: 4, label: 'ややあてはまる' },
  { value: 5, label: 'あてはまる' },
] as const

/** 状態は色に頼らず、記号と文言の両方で示す。 */
export const STATUS_DISPLAY: Record<AssessmentStatus, { mark: string; label: string }> = {
  not_started: { mark: '○', label: '未受検' },
  in_progress: { mark: '◐', label: '受検中' },
  submitted: { mark: '●', label: '提出済み' },
}

export class AssessmentApiError extends Error {
  constructor(readonly status: number, message: string) {
    super(message)
  }
}

export async function assessmentApi<T>(path: string, init?: { method: 'POST' | 'PUT'; body?: unknown }): Promise<T> {
  const res = await fetch(`/api/assessments${path}`, {
    method: init?.method ?? 'GET',
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
    body: init?.body === undefined ? undefined : JSON.stringify(init.body),
  })
  if (!res.ok) {
    const body: { error?: string } = await res.json().catch(() => ({}))
    throw new AssessmentApiError(res.status, body.error || '通信に失敗しました')
  }
  return (res.status === 204 ? undefined : await res.json()) as T
}

/** 再開時は最初に未回答の問題があるページから始める。全部答えていれば最後のページ。 */
export function firstUnansweredPage(session: AssessmentSession): number {
  const idx = session.questions.findIndex((q) => session.answers[String(q.id)] === undefined)
  const pages = Math.max(1, Math.ceil(session.questions.length / QUESTIONS_PER_PAGE))
  return idx === -1 ? pages - 1 : Math.floor(idx / QUESTIONS_PER_PAGE)
}

/**
 * 期限の表示用。年・日付・時刻まで出す。
 * 日付だけだと当日の締切時刻が分からず、年が無いと年明けの期限を今年と読み違える。
 */
export function formatDueAt(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString('ja-JP', { year: 'numeric', month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}
