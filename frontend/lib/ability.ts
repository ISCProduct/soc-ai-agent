import { authService } from '@/lib/auth'

/**
 * 能力検査（言語・非言語）のクライアント（SOCAIAGENT-433）。
 *
 * 性格検査（lib/assessments.ts）とは別にする。採点方式も結果の意味も違い、
 * 能力検査の点数は企業のおすすめ計算には使われない。
 * 型に正解（correct_choice）を持たせない。サーバも返さないが、
 * 将来レスポンスへ混ざったときに型で気づけるようにしておく。
 */

/** expired は受検中のまま制限時間を過ぎたもの。再開できない。 */
export type AbilityStatus = 'not_started' | 'in_progress' | 'submitted' | 'expired'

export type AbilitySection = 'verbal' | 'numerical'

export const SECTION_LABEL: Record<AbilitySection, string> = {
  verbal: '言語',
  numerical: '非言語',
}

export interface AbilitySummary {
  template_id: number
  title: string
  description: string
  question_count: number
  /** 制限時間（秒）。無ければ無制限 */
  time_limit_seconds?: number
  status: AbilityStatus
  session_id?: string
  answered_count: number
  submitted_at?: string
}

export interface AbilityQuestion {
  id: number
  position: number
  section: AbilitySection
  body: string
  choices: string[]
}

export interface AbilitySessionView {
  session_id: string
  title: string
  status: 'in_progress' | 'submitted'
  questions: AbilityQuestion[]
  /** question_id -> 選択肢の添字（JSON のキーなので文字列） */
  answers: Record<string, number>
  /** 締め切り（ISO）。表示用。判定はサーバが行う */
  deadline_at?: string
  /** 残り秒。表示用 */
  remaining_seconds?: number
}

export interface AbilitySectionScore {
  correct: number
  total: number
}

export interface AbilityResult {
  session_id: string
  title: string
  sections: Record<string, AbilitySectionScore>
  /** 表示順。サーバが決める（map の順序に依存させない） */
  order: AbilitySection[]
  submitted_at?: string
}

/** 1画面に出す問題数。能力検査は1問ずつ（1問に集中させ、残り時間の判断をしやすくする）。 */
export const QUESTIONS_PER_PAGE = 1

/** 状態は色に頼らず、記号と文言の両方で示す。 */
export const STATUS_DISPLAY: Record<AbilityStatus, { mark: string; label: string }> = {
  not_started: { mark: '○', label: '未受検' },
  in_progress: { mark: '◐', label: '受検中' },
  submitted: { mark: '●', label: '提出済み' },
  expired: { mark: '×', label: '時間切れ' },
}

export class AbilityApiError extends Error {
  constructor(readonly status: number, message: string) {
    super(message)
  }
}

export async function abilityApi<T>(path: string, init?: { method: 'POST' | 'PUT'; body?: unknown }): Promise<T> {
  const res = await fetch(`/api/ability-assessments${path}`, {
    method: init?.method ?? 'GET',
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
    body: init?.body === undefined ? undefined : JSON.stringify(init.body),
  })
  if (!res.ok) {
    const body: { error?: string } = await res.json().catch(() => ({}))
    throw new AbilityApiError(res.status, body.error || '通信に失敗しました')
  }
  return (res.status === 204 ? undefined : await res.json()) as T
}

/** 再開時は最初に未回答の問題から始める。全部答えていれば最後の問題。 */
export function firstUnansweredPage(session: AbilitySessionView): number {
  const idx = session.questions.findIndex((q) => session.answers[String(q.id)] === undefined)
  const pages = Math.max(1, Math.ceil(session.questions.length / QUESTIONS_PER_PAGE))
  return idx === -1 ? pages - 1 : Math.floor(idx / QUESTIONS_PER_PAGE)
}

/**
 * 残り時間の表示。分:秒。
 * 0 を下回ったら 0 にする（負の残り時間は「まだ出せる」と誤読される）。
 */
export function formatRemaining(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds))
  const m = Math.floor(s / 60)
  return `${m}:${String(s % 60).padStart(2, '0')}`
}

/**
 * 制限時間の表示（一覧用）。
 * 端数を丸めると制限時間を実際より長く・短く見せるので、残りの秒まで出す。
 */
export function formatTimeLimit(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  if (m === 0) return `${s}秒`
  return s === 0 ? `${m}分` : `${m}分${s}秒`
}
