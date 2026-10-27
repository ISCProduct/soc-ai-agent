import { STUDENT_SURFACES, surfaceVar, type StudentSurface } from '../student-theme'

/**
 * 学生向け画面の地・面・文字・罫線（#1572）。
 *
 * 値の出どころは lib/student-theme.ts の STUDENT_SURFACES に一本化した。
 * 以前は /results だけが独自の灰緑(#E5E8DF)を持ち、他の画面はテーマの #f4f7fb で
 * 別製品のように見えていた。テーマ側を正とした。
 *
 * 値は CSS 変数で、テーマ（MuiCssBaseline の :root）が表示設定に応じて差し替える。
 * テーマの外で描画されたとき（単体テスト等）は既定配色にフォールバックする。
 * アクセントは Wong 配色（色覚セーフ）。ここは意匠ではなく要件なので変えない。
 */
export const UI = Object.fromEntries(
  Object.entries(STUDENT_SURFACES.comfortable).map(([k, v]) => [k, `var(${surfaceVar(k as keyof StudentSurface)}, ${v})`]),
) as Record<keyof StudentSurface, string>

/**
 * 本文の列幅。ヘッダーの帯と本文で同じ値を使い、列の左端を揃える。
 *
 * 揃えないと、帯だけ画面幅いっぱいに左寄せされ、本文は中央寄せになるため、
 * 広い画面ほど見出しと中身がずれて壊れて見える（2000px 幅で 376px のずれ）。
 * 既に /es-rewrite・企業詳細・管理画面も同じ 1200 を使っている。
 */
export const CONTENT_MAX_WIDTH = 1200

/**
 * 管理画面とは別の画面なので、幅の変更が相互に波及しないよう専用の定数にする。
 */
export const INTERVIEW_HISTORY_CONTENT_MAX_WIDTH = 1100

/**
 * 適合度の共通スケール。結果画面でのみ使う。
 *
 * 数値を単独で大きく出すと、78 と 71 の差が実際より大きく見える。
 * 実測では公開企業のマッチ度が 60〜74 に密集しており（#1331）、
 * 並びの多くは僅差でしかない。候補全体の幅を背景に置き、その中のどこに
 * 位置するかを見せることで、差の小ささも含めて正直に伝える。
 */
export function scaleBand(scores: number[]): { min: number; max: number } {
  if (scores.length === 0) return { min: 0, max: 100 }
  const min = Math.min(...scores)
  const max = Math.max(...scores)
  // 全社同点だと幅が 0 になり位置が定まらないので、最低限の幅を与える
  if (max - min < 1) return { min: Math.max(0, min - 1), max: min + 1 }
  return { min, max }
}

/** 候補全体の幅の中での位置（0〜1）。 */
export function positionInBand(score: number, band: { min: number; max: number }): number {
  const span = band.max - band.min
  if (span <= 0) return 0.5
  return Math.min(1, Math.max(0, (score - band.min) / span))
}

/**
 * 理由文の先頭だけを見せる。
 *
 * 生成された理由文はテンプレートに沿って5段落ぶん続き、後半は
 * 「強みが発揮できる場面が多く…」のようにどの企業でも同じ文になる。
 * 全文をそのまま流すと、読む側は毎回同じ定型を読まされ、結果として
 * どこも同じに見える。企業ごとに違う前半だけを出す。
 *
 * 続きは企業詳細で読めるので、ここで落としても情報は失われない。
 */
export function leadSentences(text: string | undefined, max = 2): string {
  if (!text) return ''
  const normalized = text.replace(/\s+/g, ' ').trim()
  // 句点で切る。末尾の句点は残す
  const parts = normalized.match(/[^。]+。?/g)
  if (!parts) return normalized
  const lead = parts.slice(0, max).join('').trim()
  return lead || normalized
}
