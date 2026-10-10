import { createTheme } from '@mui/material/styles'
import { COMPANY_COLORS, FONT_STACK_PROPORTIONAL } from './design-tokens'

/**
 * 企業ポータルのテーマ。
 *
 * 学生画面の青でも管理画面の藍でもない。採用担当者が「次に何をするか」を
 * 読むための密度にし、角丸は業務画面として 4px に抑える。
 */
export function createCompanyMuiTheme() {
  return createTheme({
    palette: {
      mode: 'light',
      primary: { main: COMPANY_COLORS.forest, contrastText: '#FFFFFF' },
      secondary: { main: COMPANY_COLORS.amber, contrastText: '#FFFFFF' },
      error: { main: COMPANY_COLORS.seal, contrastText: '#FFFFFF' },
      warning: { main: COMPANY_COLORS.amber, contrastText: '#FFFFFF' },
      success: { main: COMPANY_COLORS.forest, contrastText: '#FFFFFF' },
      background: { default: COMPANY_COLORS.ground, paper: COMPANY_COLORS.paper },
      text: { primary: COMPANY_COLORS.ink, secondary: COMPANY_COLORS.muted },
      divider: COMPANY_COLORS.rule,
    },
    typography: {
      fontFamily: FONT_STACK_PROPORTIONAL,
      fontSize: 16,
      body1: { fontSize: '1rem', lineHeight: 1.6 },
      body2: { fontSize: '1rem', lineHeight: 1.6 },
      caption: { fontSize: '0.875rem', lineHeight: 1.5 },
      button: { textTransform: 'none', fontWeight: 700 },
      h4: { fontSize: '1.75rem', fontWeight: 700, letterSpacing: '-0.01em' },
      h6: { fontSize: '1.125rem', fontWeight: 700 },
    },
    shape: { borderRadius: 4 },
    components: {
      MuiPaper: {
        defaultProps: { elevation: 0 },
        styleOverrides: { root: { backgroundImage: 'none' } },
      },
      MuiButton: {
        defaultProps: { disableElevation: true },
        styleOverrides: {
          root: { minHeight: 44, whiteSpace: 'nowrap', flexShrink: 0 },
          sizeSmall: { minHeight: 36 },
        },
      },
      MuiTable: {
        styleOverrides: { root: { minWidth: 640 } },
      },
      MuiTableCell: {
        styleOverrides: {
          root: {
            fontSize: '1rem',
            color: COMPANY_COLORS.ink,
            borderBottomColor: COMPANY_COLORS.rule,
            whiteSpace: 'nowrap',
          },
          head: {
            fontWeight: 700,
            fontSize: '0.875rem',
            color: COMPANY_COLORS.ink,
            backgroundColor: COMPANY_COLORS.ground,
            borderBottom: `1px solid ${COMPANY_COLORS.rule}`,
          },
          sizeSmall: { paddingTop: 12, paddingBottom: 12 },
        },
      },
      MuiCssBaseline: {
        styleOverrides: {
          body: { fontFamily: FONT_STACK_PROPORTIONAL, backgroundColor: COMPANY_COLORS.ground },
          '*:focus-visible': {
            outline: `2px solid ${COMPANY_COLORS.forest}`,
            outlineOffset: 2,
          },
        },
      },
    },
  })
}
