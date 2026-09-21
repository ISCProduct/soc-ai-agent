/**
 * 学生向け画面の配色。
 *
 * 学生が受け取るのは「応募先の候補リスト」であって、ダッシュボードではない。
 * 学校の就職課で配られる一覧に近い見え方を狙い、地の色を淡い灰緑にしている。
 * SaaS既定の寒色グレー(#f4f7fb)でも、生成AIの定番になったクリーム系でもない。
 *
 * アクセントは既存の Wong 配色（色覚セーフ）をそのまま使う。ここは意匠ではなく
 * 要件なので変えない。地の色だけを替えて印象を動かす。
 */
export const UI = {
  /** 地。藁半紙寄りの淡い灰緑 */
  paper: '#E5E8DF',
  /** 本文が乗る面 */
  field: '#FCFCFA',
  ink: '#1C201D',
  inkSoft: '#545C56',
  /** 罫線 */
  rule: '#B9BEB2',
  ruleSoft: '#D5D8CE',
  /** 既存の Wong 青（色覚セーフ）。順位・操作の色 */
  mark: '#0072B2',
  /** 要確認だけに使う。紙に合わせて彩度を落とした vermillion */
  flag: '#B44A00',
} as const

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
