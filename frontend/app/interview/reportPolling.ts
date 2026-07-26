/**
 * 面接レポート生成ポーリングの定数と判定ロジック。
 * UI / hook から分離し、タイムアウト判定を単体テスト可能にする。
 */

export const REPORT_POLL_INTERVAL_MS = 3000
/** レポート未到着の最大待ち時間（3分） */
export const REPORT_POLL_TIMEOUT_MS = 3 * 60 * 1000

export type ReportPollTickResult = 'ready' | 'continue' | 'timeout' | 'error'

/**
 * 1回のポーリング結果を評価する。
 * - fetch 失敗 → error
 * - report あり → ready
 * - 開始から timeoutMs 経過 → timeout
 * - それ以外 → continue（次のポーリングへ）
 */
export function evaluateReportPollTick(args: {
  startedAtMs: number
  nowMs: number
  timeoutMs?: number
  hasReport: boolean
  fetchFailed: boolean
}): ReportPollTickResult {
  if (args.fetchFailed) return 'error'
  if (args.hasReport) return 'ready'
  const timeoutMs = args.timeoutMs ?? REPORT_POLL_TIMEOUT_MS
  if (args.nowMs - args.startedAtMs >= timeoutMs) return 'timeout'
  return 'continue'
}
