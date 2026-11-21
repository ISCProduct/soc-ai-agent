/**
 * ホーム（相談中心）の配色と寸法。Figma「ホーム 改善案」の「改善G／相談中心」の値をそのまま写したもの
 * （ノード 37:339 / 37:403 / 37:340 / 37:435、2026-11-18 取得）。
 *
 * 既存のデザイントークン（lib/design/tokens.ts の UI.*。色覚セーフの Wong 配色）とは別の配色。
 * 「Figma の色をそのまま使う」と決めたので（SOCAIAGENT-668）、ホームの部品だけがここを参照する。
 * 他の画面へ持ち出さないこと。文字と背景の組み合わせは tests/lib/home-theme.test.ts でコントラストを固定している。
 */
export const HOME_COLORS = {
  /** 本文・見出し */
  ink: '#222',
  /** サイドバーの項目の文字 */
  navInk: '#1a1a1a',
  /** 補助の文字 */
  soft: '#5e5a54',
  /** ボタン・現在地 */
  primary: '#0017c1',
  /** リンク・ラベル・アウトラインボタン */
  link: '#1f5f8b',
  /** カードと入力の外枠 */
  border: '#dddad3',
  /** サイドバーの右の罫線 */
  sidebarBorder: '#ccc',
  /** 入力欄の枠 */
  inputBorder: '#666',
  /** 現在地の項目の背景 */
  activeBg: '#e8f1fe',
  /** 進捗バーの溝 */
  track: '#e3e0d9',
  /** 先生からの案内の背景と文字 */
  noticeBg: '#fff6df',
  noticeInk: '#5c4300',
  /** 画面の地 */
  page: '#f7f7f5',
  white: '#fff',
} as const

/** 3カラムの寸法（PC 幅 1440px 想定）。 */
export const HOME_LAYOUT = {
  sidebarWidth: 280,
  panelWidth: 360,
  /** メインの本文の幅（メイン 800px から左右 40px の余白を引いた値） */
  contentWidth: 720,
  /** 右パネルを出す幅。これより狭いときは、従来どおり上に積む。 */
  panelBreakpoint: 'lg',
} as const
