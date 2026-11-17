import type { InterviewReport } from '@/lib/interview'
import type { ReportStatus } from '../../hooks/useInterviewSession'
import type { ReportProgressView } from '../../reportProgress'

/**
 * 面接後画面の「今どの見た目か」。
 * - idle: レポート生成がまだ始まっていない（finishSession 失敗中など）
 * - fallback: 生成中だが進捗が取れない（旧バックエンド）。段階リスト無しの不定表示
 * - progress: 段階・%つきの生成中
 * - stalled: 生成中だが同じ段階が長く続いている
 * - failed: 生成に失敗（サーバー確定・タイムアウト・通信失敗）
 * - ready: レポートが届いた
 */
export type ReportViewKind = 'idle' | 'fallback' | 'progress' | 'stalled' | 'failed' | 'ready'

export type ResolveReportViewInput = {
  reportStatus: ReportStatus
  reportProgress?: ReportProgressView | null
  report: InterviewReport | null
}

export function resolveReportView({ reportStatus, reportProgress, report }: ResolveReportViewInput): ReportViewKind {
  if (reportStatus === 'ready' && report) return 'ready'
  if (reportStatus === 'error' || reportStatus === 'timeout') return 'failed'
  // ready でも report が無い間は、本体を出せないので生成中として扱う
  if (reportStatus === 'pending' || reportStatus === 'ready') {
    if (!reportProgress) return 'fallback'
    return reportProgress.stalled ? 'stalled' : 'progress'
  }
  return 'idle'
}

/** 失敗の詳細文。サーバー確定の失敗は「回答は保存済み」を伝え、他と文言を分ける */
export function resolveFailureDetail(reportStatus: ReportStatus, failedByServer: boolean): string {
  if (failedByServer) return '回答はすべて保存されています。もう一度試すと、最初から作り直します。'
  if (reportStatus === 'timeout') return 'レポート生成がタイムアウトしました。時間をおいて再試行してください。'
  return 'レポート生成に失敗しました。'
}

export function resolveRetryLabel(failedByServer: boolean): string {
  return failedByServer ? 'もう一度試す' : '再試行'
}

/** レポート要約の最初の一文。講評カードに一言だけ出す。取れなければ null */
export function firstSentence(text: string | undefined | null): string | null {
  const t = (text ?? '').trim()
  if (!t) return null
  const end = t.search(/[。\n]/)
  return end < 0 ? t : t.slice(0, end + 1).trim()
}

/** 秒 → 「14分32秒」。60秒未満は「32秒」 */
export function formatDurationJa(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds))
  const m = Math.floor(s / 60)
  return m > 0 ? `${m}分${s % 60}秒` : `${s}秒`
}

/** 秒 → 「1:15」（経過表示） */
export function formatClock(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

export type InterviewMetaInput = {
  questionCount?: number
  durationSeconds?: number
  companyName?: string
  positionTitle?: string
}

/**
 * 「質問8問・14分32秒　企業名／職種」。取れない項目は出さない（捏造しない）。
 * 何も取れなければ null。
 */
export function buildMetaLine({ questionCount, durationSeconds, companyName, positionTitle }: InterviewMetaInput): string | null {
  const stats = [
    questionCount && questionCount > 0 ? `質問${questionCount}問` : null,
    durationSeconds && durationSeconds > 0 ? formatDurationJa(durationSeconds) : null,
  ].filter(Boolean).join('・')
  const target = [companyName, positionTitle].filter(Boolean).join(' ／ ')
  return [stats, target].filter(Boolean).join('　') || null
}
