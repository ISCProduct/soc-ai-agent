// マッチ度の低い企業への応募を、学生に一度だけ気づかせるための判定（#1028）。
//
// ブロックはしない。応募するかどうかは学生の意思決定であり、
// ここは「気づきのきっかけ」に留める（DesignDoc のガードレール）。

/** この値を下回ると確認ダイアログを出す。境界値ちょうどは出さない。 */
export const LOW_MATCH_THRESHOLD = 40

/**
 * 確認ダイアログを出すべきかを返す。
 *
 * スコアが未設定・数値でない場合は出さない。
 * 「スコアが取れていない」ことと「スコアが低い」ことは違い、
 * 前者で警告を出すと、算出前の企業すべてに警告が出てしまう。
 */
export function needsLowMatchConfirm(matchScore: number | null | undefined): boolean {
  if (!Number.isFinite(matchScore)) return false
  return (matchScore as number) < LOW_MATCH_THRESHOLD
}
