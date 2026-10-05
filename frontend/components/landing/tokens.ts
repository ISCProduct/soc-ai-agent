/**
 * 公開LP専用の配色・モーショントークン（#1653）。
 *
 * 画面内の primary は学生テーマの `#0072B2`（Wong の色覚セーフ青）をそのまま使う。
 * 既存画面と別の青を持ち込むと、LPからログインした瞬間に色が変わる。
 *
 * ただしBtoB SaaSのLPは青が圧倒的多数で埋もれやすい。差別化はキーカラーを
 * 変えるのではなく、**濃い地のファーストビュー**と、Wong の橙 `#E69F00` を
 * 強調に使うことで作る。橙も学生テーマが warning として持っている値で、
 * 色覚セーフの組み合わせを崩さない。
 *
 * `BRAND_LOGO_COLOR`（#ec5b13）は使わない。`lib/brand.ts` が
 * 「ロゴ・OGP専用。画面の primary には使わない（#853）」と定めている。
 */
export const LP = {
  /** 濃色の地。ファーストビューとクロージングに使う。 */
  ink: '#0C1620',
  /** ink より一段明るい。濃色セクション内の面に使う。 */
  inkSoft: '#152433',
  paper: '#FFFFFF',
  /** 薄い地。セクションの切り替えに使う。 */
  tint: '#EEF4F9',
  rule: '#D4DEE7',
  muted: '#5A6B7A',
  /** 主操作。学生テーマの COMFORTABLE_PRIMARY と同じ値。 */
  primary: '#0072B2',
  primaryHover: '#00598B',
  /** 濃色の上で使う明るい青。#0072B2 は濃色地だとコントラストが足りない。 */
  primaryOnDark: '#56B4E9',
  /** 強調。Wong の橙。学生テーマの warning と同じ値。 */
  accent: '#E69F00',
} as const

/**
 * 読み込み時のフェードアップ。スクロール連動にはしない。
 * LandingContent は Server Component のままにしたいので、JS を足さずCSSだけで出す。
 * `prefers-reduced-motion` を尊重すること（各所で指定している）。
 */
export const RISE = {
  '@keyframes lpRise': {
    from: { opacity: 0, transform: 'translateY(14px)' },
    to: { opacity: 1, transform: 'none' },
  },
  animation: 'lpRise .7s cubic-bezier(.22,.61,.36,1) both',
  '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
} as const

/** 段差をつけて順に出す。 */
export const delay = (i: number) => ({ animationDelay: `${0.08 * i}s` })
