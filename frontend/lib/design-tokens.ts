/**
 * 3ロール（学生 / 管理者 / 企業担当者）共通のデザイントークン。
 *
 * これまで学生画面だけが根拠のあるテーマ（`student-theme.ts`: BIZ UDPGothic +
 * Wong 色覚セーフパレット）を持ち、管理画面は `primary: '#1976d2'` のみ、
 * つまり MUI 既定のままだった。企業ポータルは学生の青とも管理画面の藍とも
 * 分ける `COMPANY_COLORS` を持ち、`company-theme.ts` がそれをテーマにする。
 *
 * ここはその土台を1か所にまとめる層で、テーマの生成は各 *-theme.ts が行う。
 *
 * ## 管理画面を別 identity にしている理由
 *
 * プロジェクトルール §29 は「1画面だけ独自デザインにしない」としているが、
 * 管理画面については意図的に別系統にする判断を取っている（2026-10-01 合意）。
 * 管理者の利用文脈（情報密度・作業効率）が学生（迷わせない・モバイル中心）とは
 * 別物で、同じ余白と型階層では一覧が成立しないため。ロール内の統一は保つ。
 *
 * ## 書体の扱い
 *
 * BIZ UD 系は webfont として配信しておらず family 指定だけなので、
 * 未インストール環境ではフォールバックに落ちて学生側と同じ字面になる。
 * identity を書体に依存させず、配色・密度・鮮度ガターで担保すること。
 */

/** 本文の基準サイズ。プロジェクトルール §20 の「16px前後」。 */
export const BODY_FONT_SIZE = 16

/**
 * 学生・企業担当者向けの書体。
 * BIZ UDPGothic は学校配布物・自治体文書のUD書体で、専門学校生が日頃見ている字面。
 */
export const FONT_STACK_PROPORTIONAL =
  "'BIZ UDPGothic', 'Noto Sans JP', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif"

/**
 * 管理画面向けの書体。
 * 同じ BIZ UD ファミリの非プロポーショナル版。判別性の保証は維持したまま、
 * 密な一覧での字送りが変わる。
 */
export const FONT_STACK_TABULAR =
  "'BIZ UDGothic', 'BIZ UDPGothic', 'Noto Sans JP', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif"

/**
 * 管理画面の配色。日本の台帳・登記簿の「罫線の墨・紙・朱印」を土台にする。
 *
 * 学生側 primary (#0072B2) と MUI 既定 (#1976d2) の両方と明確に分けている。
 * 判別は色だけに依存させない（§19）。状態は必ず字形か文字を伴わせる。
 */
export const ADMIN_COLORS = {
  /** 本文。意図的に色味のある濃色。#111 を黒代わりに使わない。 */
  ink: '#26303D',
  /** 罫線。カードと影の代わりに構造を担う（§18）。 */
  rule: '#C6D0DB',
  paper: '#FFFFFF',
  /** ゼブラ・左ガターの地。 */
  rail: '#EEF2F7',
  /** 主操作。 */
  indigo: '#2F4B7C',
  /**
   * 副次操作。「情報を取得」のような、主操作ではないが実行を促す操作に使う。
   *
   * 定義しないと MUI 既定の secondary（紫 #9c27b0）が出て、
   * 管理画面の配色から1色だけ浮く。管理画面では94箇所が secondary を参照している。
   * 白文字を載せて 6.15:1 で、indigo とは色相で分かれる。
   */
  teal: '#2A6B6B',
  /** 破壊的操作と期限切れだけに使う。 */
  seal: '#A4303F',
  /** 補助文字。白地で 4.5:1 を満たす。 */
  muted: '#5B6B7C',
} as const

/**
 * 企業ポータルの配色。
 *
 * 学生の primary (#0072B2) と管理画面の indigo / teal のどちらとも分ける。
 * 主操作は採用パイプラインの終着（内定承諾）に寄せた緑。
 * 未対応の滞留にだけ amber、破壊的操作は管理画面と同じ seal。
 * 構造は影ではなく罫線（§18）。状態は色だけで伝えず、字形と文字を伴わせる（§19）。
 */
export const COMPANY_COLORS = {
  /** 主操作。白文字を載せてコントラストを取る。 */
  forest: '#1F5136',
  /** 未対応・滞留。放置すると困る状態の文字色にだけ使う。 */
  amber: '#8A5A14',
  /** 破壊的操作。管理画面の seal と同じ値。 */
  seal: ADMIN_COLORS.seal,
  /** 画面の地。管理画面の寒色 rail と対になる温かい紙。 */
  ground: '#F4F3EF',
  paper: '#FFFFFF',
  /** 本文。純黒ではなく緑に寄せた濃色。 */
  ink: '#23282A',
  /** 罫線。 */
  rule: '#D7D6CE',
  /** 補助文字。紙の上で 4.5:1 を満たす濃さ。 */
  muted: '#3E4844',
} as const

/**
 * データ鮮度の段階。管理画面の一覧で最も繰り返し現れる意味づけ。
 *
 * 取得日時の TTL（info 90日 / relations 60日 / tech 30日 / jobs 7日）と
 * 欠損の有無がこの画面群の主要な判断材料なので、汎用のステータス配色ではなく
 * 鮮度を第一級のトークンとして持つ。
 */
export type Freshness = 'fresh' | 'due' | 'expired' | 'unfetched'

/**
 * 鮮度の表示定義。`mark` は色を読めない環境でも区別がつくようにするための字形で、
 * 色と併用する前提（§19）。`label` は読み上げと文字表示の両方に使う。
 */
export const FRESHNESS: Record<Freshness, { mark: string; label: string; color: string }> = {
  fresh: { mark: '', label: '取得済み', color: ADMIN_COLORS.ink },
  due: { mark: '◐', label: '期限が近い', color: '#7A5A12' },
  expired: { mark: '●', label: '期限切れ', color: ADMIN_COLORS.seal },
  unfetched: { mark: '○', label: '未取得', color: ADMIN_COLORS.muted },
}

/**
 * 取得日時と TTL から鮮度を決める。
 *
 * `fetchedAt` が無ければ未取得。TTL を過ぎていれば期限切れ。
 * 残りが `dueRatio` を下回ったら「期限が近い」。
 *
 * 「取得したが空だった」と「まだ取得していない」はこの関数では区別できない。
 * 区別には取得試行の記録が必要で、現状バックエンドに列が無い（調査済み）。
 * その間は `fetchedAt` があれば取得済みとして扱い、欠損は別の列で示すこと。
 */
export function resolveFreshness(
  fetchedAt: Date | string | null | undefined,
  ttlDays: number,
  now: Date = new Date(),
  dueRatio = 0.2,
): Freshness {
  if (!fetchedAt) return 'unfetched'
  const at = fetchedAt instanceof Date ? fetchedAt : new Date(fetchedAt)
  if (Number.isNaN(at.getTime())) return 'unfetched'

  const ageDays = (now.getTime() - at.getTime()) / 86_400_000
  if (ageDays >= ttlDays) return 'expired'
  if (ageDays >= ttlDays * (1 - dueRatio)) return 'due'
  return 'fresh'
}

/** バックエンド `companyfetch` の TTL と対応させる（`text.go`）。 */
export const TTL_DAYS = {
  info: 90,
  relations: 60,
  tech: 30,
  jobs: 7,
} as const

/**
 * 管理画面の型階層。4段に固定する。
 * 段を増やすと一覧の情報量に対して見出しが勝ってしまう。
 */
export const ADMIN_TYPE_SCALE = {
  caption: 13,
  body: 15,
  section: 20,
  page: 28,
} as const

/**
 * 数値セルに当てるスタイル。法人番号・金額・トークン数を桁で揃える。
 *
 * 小さいラベルを等幅書体にするのとは別で、こちらは桁合わせという機能上の必要。
 * 同じ family のまま数字だけ等幅にするので字面が混ざらない。
 */
export const TABULAR_NUMS = {
  fontVariantNumeric: 'tabular-nums',
  fontFeatureSettings: '"tnum"',
} as const
