/**
 * 面接レポート生成ポーリングの定数と判定ロジック。
 * UI / hook から分離し、タイムアウト判定を単体テスト可能にする。
 */

export const REPORT_POLL_INTERVAL_MS = 3000
/** レポート未到着の最大待ち時間（3分） */
export const REPORT_POLL_TIMEOUT_MS = 3 * 60 * 1000

/**
 * error に落とすまでに許容する「連続」fetch 失敗回数（#1057）。
 * ネットワーク瞬断の1回でポーリングを止めないための閾値。
 * 3秒間隔なので、3回連続 = 約6〜9秒は復帰を待つ。
 */
export const REPORT_POLL_MAX_CONSECUTIVE_FAILURES = 3

export type ReportPollTickResult = 'ready' | 'continue' | 'timeout' | 'error'

/**
 * 1回のポーリング結果を評価する。
 * - report あり → ready（過去に失敗があっても、取得できた結果を優先する）
 * - 連続失敗が閾値以上 → error
 * - 開始から timeoutMs 経過 → timeout
 * - それ以外 → continue（次のポーリングへ）
 *
 * consecutiveFailures は「連続」失敗数。fetch が成功したら呼び出し側で 0 に戻すこと。
 */
export function evaluateReportPollTick(args: {
  startedAtMs: number
  nowMs: number
  timeoutMs?: number
  hasReport: boolean
  consecutiveFailures: number
  maxConsecutiveFailures?: number
}): ReportPollTickResult {
  if (args.hasReport) return 'ready'
  const maxFailures = args.maxConsecutiveFailures ?? REPORT_POLL_MAX_CONSECUTIVE_FAILURES
  if (args.consecutiveFailures >= maxFailures) return 'error'
  const timeoutMs = args.timeoutMs ?? REPORT_POLL_TIMEOUT_MS
  if (args.nowMs - args.startedAtMs >= timeoutMs) return 'timeout'
  return 'continue'
}
