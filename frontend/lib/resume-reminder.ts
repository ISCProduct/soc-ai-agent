import { authService } from '@/lib/auth'

export interface ResumeStatus {
  has_document: boolean
  latest_score: number | null
  needs_attention: boolean
}

/** バックエンドの判定閾値のミラー。食い違ったときは表示しない側に倒す */
export const RESUME_SCORE_THRESHOLD = 60

/**
 * リマインダーの表示要否と文言を判定する純関数。表示不要なら null を返す。
 */
export function resumeReminderMessage(status: ResumeStatus): string | null {
  if (!status.needs_attention) return null
  if (!status.has_document) return '履歴書がまだ作成されていません'
  if (status.latest_score === null || status.latest_score >= RESUME_SCORE_THRESHOLD) return null
  return `履歴書の評価が低めです。改善しましょう（スコア: ${status.latest_score}）`
}

export async function fetchResumeStatus(): Promise<ResumeStatus> {
  const res = await fetch('/api/resume/status', {
    cache: 'no-store',
    headers: authService.getUserFetchHeaders(),
  })
  if (!res.ok) throw new Error('resume-status')
  return res.json() as Promise<ResumeStatus>
}
