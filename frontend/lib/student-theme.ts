import { createTheme } from '@mui/material/styles'

export const STUDENT_THEME_STORAGE_KEY = 'student-theme-mode'

export type StudentThemeMode = 'comfortable' | 'high-contrast'

export const STUDENT_THEME_LABELS: Record<StudentThemeMode, string> = {
  comfortable: '見やすい表示（推奨）',
  'high-contrast': '高コントラスト（色弱・弱視向け）',
}

const FONT_FAMILY =
  "'Noto Sans JP', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif"

/** パターン3 既定: Wong に近い色覚セーフ青 */
export const COMFORTABLE_PRIMARY = '#0072B2'

const COMFORTABLE = {
  primary: COMFORTABLE_PRIMARY,
  success: '#009E73',
  warning: '#E69F00',
  error: '#000000',
  background: '#f4f7fb',
  paper: '#ffffff',
}

/** パターン5: 高コントラスト */
const HIGH_CONTRAST = {
  primary: '#0B3D91',
  success: '#006400',
  warning: '#B36B00',
  error: '#8B0000',
  background: '#ffffff',
  paper: '#ffffff',
}

export function isStudentThemeMode(value: string | null | undefined): value is StudentThemeMode {
  return value === 'comfortable' || value === 'high-contrast'
}

export function readStudentThemeMode(storage?: Pick<Storage, 'getItem'> | null): StudentThemeMode {
  const raw = storage?.getItem(STUDENT_THEME_STORAGE_KEY)
  return isStudentThemeMode(raw) ? raw : 'comfortable'
}

export function writeStudentThemeMode(
  mode: StudentThemeMode,
  storage?: Pick<Storage, 'setItem'> | null,
): void {
  storage?.setItem(STUDENT_THEME_STORAGE_KEY, mode)
}

export function createStudentMuiTheme(mode: StudentThemeMode) {
  const p = mode === 'high-contrast' ? HIGH_CONTRAST : COMFORTABLE
  const focusWidth = mode === 'high-contrast' ? 3 : 2

  return createTheme({
    palette: {
      mode: 'light',
      primary: { main: p.primary },
      secondary: { main: p.warning },
      success: { main: p.success },
      warning: { main: p.warning },
      error: { main: p.error },
      background: { default: p.background, paper: p.paper },
      text: { primary: '#111111', secondary: '#333333' },
    },
    typography: { fontFamily: FONT_FAMILY },
    shape: { borderRadius: mode === 'high-contrast' ? 4 : 12 },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          body: { fontFamily: FONT_FAMILY },
          '*:focus-visible': {
            outline: `${focusWidth}px solid ${p.primary}`,
            outlineOffset: 2,
          },
        },
      },
      MuiButton: {
        styleOverrides: {
          root: { textTransform: 'none', fontWeight: 700, minHeight: 44 },
        },
      },
      MuiCard: {
        styleOverrides: {
          root: {
            border: mode === 'high-contrast' ? `2px solid ${p.primary}` : '1px solid #d0d7de',
          },
        },
      },
    },
  })
}
