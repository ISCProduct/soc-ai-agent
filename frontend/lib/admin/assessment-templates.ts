import { authService } from '@/lib/auth'
import type { AssessmentStatus } from '@/lib/assessments'

/** 先生向け: 学校独自の検査テンプレート（#1675）。学校独自の結果はマッチングに使われない。 */
export interface SchoolTemplate {
  template_id: number
  title: string
  description: string
  question_count: number
  due_at?: string
  created_at: string
}

export interface SchoolTemplateList {
  templates: SchoolTemplate[]
  /** 問題に付けられる正典カテゴリ。サーバーが正本で、ここに複製を持たない。 */
  categories: string[]
}

export interface StudentAssessmentStatus {
  student_id: number
  name: string
  status: AssessmentStatus
  overdue: boolean
  answered_count: number
  question_count: number
  submitted_at?: string
  /** スコアの生値は返らない。傾向の強い項目名だけ。 */
  top_categories: string[]
}

export interface StudentStatusList {
  template_id: number
  title: string
  due_at?: string
  students: StudentAssessmentStatus[]
}

export interface QuestionDraft {
  body: string
  weight_category: string
  reverse_scored: boolean
}

export interface TemplateDraft {
  title: string
  description: string
  /** datetime-local の値（"2026-11-30T23:59"）。空なら期限なし。 */
  due_at: string
  questions: QuestionDraft[]
}

export class AdminAssessmentError extends Error {
  constructor(readonly status: number, message: string) {
    super(message)
  }
}

async function call<T>(path: string, schoolId: number, init?: { method: 'POST'; body: unknown }): Promise<T> {
  const res = await fetch(`/api/admin/assessment-templates${path}?school_id=${schoolId}`, {
    method: init?.method ?? 'GET',
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json', ...authService.getAdminFetchHeaders() },
    body: init ? JSON.stringify(init.body) : undefined,
  })
  const body: unknown = await res.json().catch(() => ({}))
  if (!res.ok) {
    const msg = (body as { error?: string; message?: string }).error ?? (body as { message?: string }).message
    throw new AdminAssessmentError(res.status, msg || '通信に失敗しました')
  }
  return body as T
}

export const listSchoolTemplates = (schoolId: number) => call<SchoolTemplateList>('', schoolId)

export const listStudentStatuses = (schoolId: number, templateId: number) =>
  call<StudentStatusList>(`/${templateId}/students`, schoolId)

/** datetime-local（タイムゾーン無し）を、閲覧者のローカル時刻として ISO に直す。 */
export function dueAtToISO(local: string): string | undefined {
  if (!local) return undefined
  const d = new Date(local)
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString()
}

export const createSchoolTemplate = (schoolId: number, draft: TemplateDraft) =>
  call<SchoolTemplate>('', schoolId, {
    method: 'POST',
    body: {
      title: draft.title,
      description: draft.description,
      due_at: dueAtToISO(draft.due_at),
      questions: draft.questions,
    },
  })

/** 作成前の入力チェック。サーバーも同じ規則で検証する（ここは早く気づかせるため）。 */
export function validateDraft(draft: TemplateDraft): string | null {
  if (!draft.title.trim()) return 'タイトルを入力してください'
  if (draft.questions.length < 1) return '問題を1問以上追加してください'
  for (const [i, q] of draft.questions.entries()) {
    if (!q.body.trim()) return `${i + 1}問目の問題文を入力してください`
    if (!q.weight_category) return `${i + 1}問目のカテゴリを選んでください`
  }
  return null
}

export const STATUS_SORT_HINT = '期限超過 → 未受検 → 途中 → 完了の順に並んでいます'
