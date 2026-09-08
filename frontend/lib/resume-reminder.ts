import { authService } from '@/lib/auth'

export interface ResumeStatus {
  has_document: boolean
  latest_score: number | null
  needs_attention: boolean
}

/**
 * リマインダーの表示要否と文言を判定する純関数。表示不要なら null を返す。
 *
 * 閾値判定は行わない。閾値は RESUME_COMPLETENESS_THRESHOLD で運用中に変更されうるため、
 * バックエンドの needs_attention を唯一の判断根拠とする。ここで閾値をミラーすると、
 * 例えば閾値を75へ上げたときにスコア70が握り潰される。
 */
export function resumeReminderMessage(status: ResumeStatus): string | null {
  if (!status.needs_attention) return null
  if (!status.has_document) return '履歴書がまだ作成されていません'
  // 提出済みだがレビュー未実施。レビューは学生が明示的に実行する操作なので、
  // 「待ってください」ではなく実行を促す文言にする。
  if (status.latest_score === null) return '履歴書のレビューをまだ受けていません'
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
