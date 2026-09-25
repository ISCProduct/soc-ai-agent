// マッチ度の低い企業への応募を、学生に一度だけ気づかせるための判定（#1028）。
//
// ブロックはしない。応募するかどうかは学生の意思決定であり、
// ここは「気づきのきっかけ」に留める（DesignDoc のガードレール）。

/** この値を下回ると確認ダイアログを出す。境界値ちょうどは出さない。 */
export const LOW_MATCH_THRESHOLD = 40

/**
 * マッチ度を信用するのに最低限必要な軸の数（#1124）。
 *
 * マッチ度は「計測できた軸だけ」の平均なので、2軸しか埋まっていない学生は
 * 実力ではなく計測不足で低い値になる。そこに確認ダイアログを出すと
 * 「スコアが取れていないこと」を「スコアが低いこと」として扱ってしまう。
 * 下の needsLowMatchConfirm が元から避けていたのと同じ誤りなので、同じ扱いにする。
 */
export const MIN_MATCHED_AXES_FOR_LOW_MATCH = 4

/**
 * 確認ダイアログを出すべきかを返す。
 *
 * スコアが未設定・数値でない場合は出さない。
 * 「スコアが取れていない」ことと「スコアが低い」ことは違い、
 * 前者で警告を出すと、算出前の企業すべてに警告が出てしまう。
 *
 * matchedAxisCount が分かる場合、軸が少なすぎる行も同じ理由で出さない。
 * 未指定（古いレスポンス）のときは従来どおりスコアだけで判定する。
 */
export function needsLowMatchConfirm(
  matchScore: number | null | undefined,
  matchedAxisCount?: number | null,
): boolean {
  if (!Number.isFinite(matchScore)) return false
  if (
    Number.isFinite(matchedAxisCount) &&
    (matchedAxisCount as number) < MIN_MATCHED_AXES_FOR_LOW_MATCH
  ) {
    return false
  }
  return (matchScore as number) < LOW_MATCH_THRESHOLD
}
