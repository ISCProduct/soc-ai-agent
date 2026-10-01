import { createTheme } from '@mui/material/styles'
import {
  ADMIN_COLORS,
  ADMIN_TYPE_SCALE,
  BODY_FONT_SIZE,
  FONT_STACK_TABULAR,
  TABULAR_NUMS,
} from './design-tokens'

/**
 * 管理画面のテーマ。
 *
 * 置き換え前は `createTheme({ palette: { primary: { main: '#1976d2' } } })` だけで、
 * 型階層・余白・角丸・コンポーネント指定がすべて MUI 既定のままだった。
 * 27画面がその状態で、共通レイアウトも無い。
 *
 * ## 方針
 *
 * 1. 構造は罫線で表す。カードと影は使わない（§18）
 * 2. 数値は等幅桁で揃える（法人番号・金額・トークン数）
 * 3. 型階層は4段に固定する
 * 4. 状態は色だけで表さない（§19）。ここでは地と罫線しか定義せず、
 *    字形は `FreshnessMark` / `StatusBadge` が担う
 *
 * ## まだ適用していない
 *
 * `MuiProvider` への接続は別手順にしてある。接続した時点で27画面の見た目が
 * 一斉に変わるため、基準画面を作って確認してから切り替える。
 */
export function createAdminMuiTheme() {
  return createTheme({
    palette: {
      mode: 'light',
      primary: { main: ADMIN_COLORS.indigo },
      secondary: { main: ADMIN_COLORS.teal },
      error: { main: ADMIN_COLORS.seal },
      background: { default: ADMIN_COLORS.rail, paper: ADMIN_COLORS.paper },
      text: { primary: ADMIN_COLORS.ink, secondary: ADMIN_COLORS.muted },
      divider: ADMIN_COLORS.rule,
    },

    typography: {
      fontFamily: FONT_STACK_TABULAR,
      fontSize: BODY_FONT_SIZE,
      // 一覧の情報密度を上げるため本文は 15px。見出しは4段だけ使う。
      body1: { fontSize: ADMIN_TYPE_SCALE.body, lineHeight: 1.6 },
      body2: { fontSize: ADMIN_TYPE_SCALE.caption, lineHeight: 1.6 },
      caption: { fontSize: ADMIN_TYPE_SCALE.caption },
      h4: { fontSize: ADMIN_TYPE_SCALE.page, fontWeight: 700, letterSpacing: '-0.02em' },
      h5: { fontSize: ADMIN_TYPE_SCALE.section, fontWeight: 700 },
      h6: { fontSize: ADMIN_TYPE_SCALE.section, fontWeight: 700 },
      // 既定の大見出しは管理画面では使わない。使われても一覧を壊さない大きさに抑える。
      h1: { fontSize: ADMIN_TYPE_SCALE.page, fontWeight: 700 },
      h2: { fontSize: ADMIN_TYPE_SCALE.page, fontWeight: 700 },
      h3: { fontSize: ADMIN_TYPE_SCALE.section, fontWeight: 700 },
      button: { textTransform: 'none', fontWeight: 600 },
    },

    // 4px。0 にすると罫線だけの紙面になり、業務画面としての手触りが硬くなりすぎる。
    shape: { borderRadius: 4 },

    components: {
      // 影ではなく罫線で面を区切る。
      MuiPaper: {
        defaultProps: { elevation: 0 },
        styleOverrides: {
          root: { backgroundImage: 'none' },
          outlined: { borderColor: ADMIN_COLORS.rule },
        },
      },

      MuiTableCell: {
        styleOverrides: {
          root: {
            borderBottomColor: ADMIN_COLORS.rule,
            // 一覧の行高を詰める。学生画面より高密度にする意図。
            paddingTop: 8,
            paddingBottom: 8,
            fontSize: ADMIN_TYPE_SCALE.body,
          },
          head: {
            fontWeight: 700,
            fontSize: ADMIN_TYPE_SCALE.caption,
            color: ADMIN_COLORS.ink,
            backgroundColor: ADMIN_COLORS.rail,
            whiteSpace: 'nowrap',
          },
          // 数値列は `align="right"` を付ければ桁が揃う。
          alignRight: TABULAR_NUMS,
        },
      },

      MuiButton: {
        defaultProps: { disableElevation: true },
        styleOverrides: {
          // タップ領域を確保する（#1481 で IconButton 34px が不足だった件と同じ基準）。
          root: { minHeight: 44 },
          sizeSmall: { minHeight: 36 },
        },
      },

      MuiIconButton: {
        styleOverrides: { root: { width: 44, height: 44 } },
      },

      // focus を必ず見えるようにする（§22）。
      MuiCssBaseline: {
        styleOverrides: {
          ':focus-visible': {
            outline: `2px solid ${ADMIN_COLORS.indigo}`,
            outlineOffset: 2,
          },
          // 本文の行長を抑える（§frontend: 80文字未満）。
          p: { maxWidth: '72ch' },
        },
      },

      MuiChip: {
        styleOverrides: {
          root: { fontSize: ADMIN_TYPE_SCALE.caption, fontWeight: 600 },
        },
      },
    },
  })
}
