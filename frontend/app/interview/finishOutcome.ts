/**
 * 面接終了(handleStop)時に表示するエラーメッセージの決定ロジック。
 * UI / hook から分離し、finishSession失敗時に握りつぶさないことを単体テスト可能にする(#1015)。
 */

export const FINISH_FAILED_MESSAGE = '面接の終了処理に失敗しました。お手数ですが再試行してください。'
export const FORCED_STOP_MESSAGE = '時間上限に達したため面接を終了しました。'

/**
 * - finishSession が失敗した場合 → 再試行を促すエラーメッセージ（最優先・握りつぶさない）
 * - 時間切れによる強制終了の場合 → その旨のメッセージ
 * - どちらでもなければ null（エラーなし）
 */
export function resolveFinishOutcomeMessage(args: { finishFailed: boolean; forced: boolean }): string | null {
  if (args.finishFailed) return FINISH_FAILED_MESSAGE
  if (args.forced) return FORCED_STOP_MESSAGE
  return null
}
