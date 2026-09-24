/**
 * 面接発話の保存（POST /utterances）の再試行と、失敗時のユーザー向けメッセージ（#1476）。
 *
 * 発話はスコア（user_weight_scores）とレポートの入力そのもの。
 * 保存が落ちたのに console.error だけで続行すると、ユーザーは面接をやり切ったのに
 * 中身のないレポートを受け取り、原因にも辿り着けない。
 * ネットワーク瞬断は短い再試行で復帰するので、まず再試行し、
 * それでも駄目なら呼び出し側が UI に出せるよう false を返す（握りつぶさない）。
 */

/** 再試行の待ち時間。合計しても面接の体感を壊さない長さに留める */
export const UTTERANCE_SAVE_RETRY_DELAYS_MS = [400, 1200] as const

/**
 * 面接終了時に、積み残した発話保存の完了を待つ上限（#1476）。
 * ここで無制限に待つと、通信が半開きのままの端末で「終了」を押しても画面が進まない。
 */
export const UTTERANCE_FLUSH_TIMEOUT_MS = 5_000

export const UTTERANCE_SAVE_FAILED_MESSAGE =
  '発話の記録に失敗しました。面接はこのまま続けられますが、記録できなかった発言はレポートとスコアに反映されません。'

const defaultSleep = (ms: number) => new Promise<void>(resolve => setTimeout(resolve, ms))

/**
 * save を最大 delaysMs.length + 1 回試す。
 * @returns 保存できたら true、全て失敗したら false
 */
export async function saveUtteranceWithRetry(
  save: () => Promise<void>,
  options: { delaysMs?: readonly number[]; sleep?: (ms: number) => Promise<void> } = {},
): Promise<boolean> {
  const delays = options.delaysMs ?? UTTERANCE_SAVE_RETRY_DELAYS_MS
  const sleep = options.sleep ?? defaultSleep

  for (let attempt = 0; ; attempt++) {
    try {
      await save()
      return true
    } catch (e) {
      if (attempt >= delays.length) {
        console.error('[utterance save error]', e)
        return false
      }
      await sleep(delays[attempt])
    }
  }
}
