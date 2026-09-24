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
 * 発話1件を識別するIDを発行する（#1476）。
 *
 * 保存の再試行は「サーバーはDBへ書けたが応答だけ失われた」失敗を含むため、
 * 同じ発話には同じIDを付けて送り直し、サーバー側の一意制約
 * (session_id, client_utterance_id) で二重保存を弾く。
 * 再試行のたびに発行し直すと意味が無いので、保存の外側で1回だけ呼ぶこと。
 *
 * crypto.randomUUID は secure context でしか生えないため、
 * 無い環境（http配信の校内端末など）でも面接が止まらないようフォールバックを持つ。
 */
export function newClientUtteranceId(): string {
  const uuid = globalThis.crypto?.randomUUID?.()
  if (uuid) return uuid
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`
}

/**
 * 積み残した発話保存を全て片付けてから終了処理（finishSession）を走らせる（#1476）。
 *
 * finishSession はレポート生成をキューする。保存が残ったまま呼ぶと、バックエンドは
 * 発話が0件ではないため不完全なログでレポートを正常に確定させ、その後キューが成功しても
 * 作り直されない。上限付きで打ち切っても結果は同じ（保存1件は再試行込みで25秒超かかりうるため、
 * 現実的な上限では足りない）なので、ここでは待ち切る。呼び出し側は待つ前に画面を先へ進めること。
 *
 * pendingSaves は必ず有限時間で解決する鎖であること（saveUtteranceWithRetry は投げず、
 * チェーン側に .catch がある）。reject する鎖を渡すと finish が実行されない。
 */
export async function flushThenFinish(
  pendingSaves: Promise<void>,
  finish: () => Promise<void>,
): Promise<void> {
  await pendingSaves
  await finish()
}

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
