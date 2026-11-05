/**
 * 公開LP専用のトークン（#1653）。
 *
 * ## 方向
 *
 * BtoB SaaS の LP で最も多い「濃紺の地＋青い放射グラデ＋同じ角丸の白カードの反復」
 * を避ける。見分けがつかず、生成物に見える。
 *
 * 代わりに題材の語彙を使う。ES（エントリーシート）の原稿用紙のマス目を地紋にし、
 * 見出しは漢数字と縦組みで送る。日本の就職活動にしか無い形で、
 * 汎用テンプレートからは出てこない。
 *
 * 色数は絞る。墨・紙・朱の3色＋操作の青。面で塗らず、罫と余白で持たせる。
 */
export const LP = {
  /** 本文。純黒を使わない。わずかに温かい墨。 */
  ink: '#1C1A17',
  /** 見出しの濃度を1段落とす。 */
  inkSoft: '#3A3732',
  /** 紙。白ではなく生成りに寄せる。 */
  paper: '#FAF8F4',
  /** 面を起こすときの白。 */
  card: '#FFFFFF',
  /** 罫。原稿用紙の罫の濃度。 */
  rule: '#DAD4C8',
  ruleSoft: '#E8E3D9',
  muted: '#6E675C',
  /** 操作。学生テーマの COMFORTABLE_PRIMARY と同じ値（Wong の色覚セーフ青）。
   *  LPで別の青を使うと、ログインした瞬間に色が変わる。 */
  primary: '#0072B2',
  primaryHover: '#00598B',
  /** 朱。印と強調だけに使う。面では塗らない。Wong の朱寄り。 */
  seal: '#B4452F',
} as const

/**
 * 原稿用紙のマス目。ES の地紋として使う。
 * 面で主張させない。罫の濃度を落として、紙の質感として効かせる。
 */
export const GENKO_GRID = (size = 28, color = '#E8E3D9') => ({
  backgroundImage: `linear-gradient(${color} 1px, transparent 1px),
                    linear-gradient(90deg, ${color} 1px, transparent 1px)`,
  backgroundSize: `${size}px ${size}px`,
})

/**
 * 読み込み時のフェードアップ。スクロール連動にはしない。
 * LandingContent は Server Component のままにしたいので、JS を足さずCSSだけで出す。
 */
export const RISE = {
  '@keyframes lpRise': {
    from: { opacity: 0, transform: 'translateY(12px)' },
    to: { opacity: 1, transform: 'none' },
  },
  animation: 'lpRise .8s cubic-bezier(.22,.61,.36,1) both',
} as const

/** 段差をつけて順に出す。 */
export const delay = (i: number) => ({ animationDelay: `${0.07 * i}s` })

/** モーションを切る指定。RISE と併せて使う。 */
export const NO_MOTION = {
  '@media (prefers-reduced-motion: reduce)': {
    animation: 'none',
    transition: 'none',
  },
} as const
