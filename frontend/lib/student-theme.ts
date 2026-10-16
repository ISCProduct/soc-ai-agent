import { createTheme } from '@mui/material/styles'
import { BODY_FONT_SIZE, FONT_STACK_PROPORTIONAL } from './design-tokens'

export const STUDENT_THEME_STORAGE_KEY = 'student-theme-mode'

export type StudentThemeMode = 'comfortable' | 'high-contrast'

export const STUDENT_THEME_LABELS: Record<StudentThemeMode, string> = {
  comfortable: '見やすい表示（推奨）',
  'high-contrast': '高コントラスト（色弱・弱視向け）',
}

// BIZ UDPGothic は学校配布物・自治体文書で使われるユニバーサルデザイン書体。
// 利用者（専門学校生）が日頃目にしている字面で、判別性も高い。
// 定義は design-tokens に一本化してある（企業ポータルも同じ書体を使うため）。
const FONT_FAMILY = FONT_STACK_PROPORTIONAL

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

/**
 * 学生画面の地・面・文字・罫線（#1572）。ここが唯一の出どころ。
 * lib/design/tokens.ts の UI はこの値を CSS 変数経由で参照するので、
 * テーマを経由しない画面（直書きしていた /interview /es-rewrite /results）も
 * 高コントラストへ追従する（#1571）。
 */
export type StudentSurface = {
  paper: string
  field: string
  tint: string
  ink: string
  inkSoft: string
  inkFaint: string
  rule: string
  ruleSoft: string
  mark: string
  flag: string
  /** 面接の暗い画面（ロビー・面接中・レポート）の地・面・文字。 */
  nightBg: string
  nightStage: string
  nightPanel: string
  nightInk: string
  nightSoft: string
  nightMuted: string
}

export const STUDENT_SURFACES: Record<StudentThemeMode, StudentSurface> = {
  comfortable: {
    paper: COMFORTABLE.background,
    field: COMFORTABLE.paper,
    tint: '#f1f5f9',
    ink: '#0f172a',
    inkSoft: '#475569',
    // 旧 #94a3b8 は白地で 2.6:1 しかなく本文に使えない。4.5:1 を満たす #64748b に寄せた。
    inkFaint: '#64748b',
    rule: '#d0d7de',
    ruleSoft: '#e2e8f0',
    mark: COMFORTABLE.primary,
    flag: '#B44A00',
    nightBg: '#221610',
    nightStage: '#606553',
    nightPanel: '#2d2e31',
    nightInk: '#e8eaed',
    nightSoft: '#bdc1c6',
    nightMuted: '#9aa0a6',
  },
  'high-contrast': {
    paper: HIGH_CONTRAST.background,
    field: HIGH_CONTRAST.paper,
    tint: '#f0f0f0',
    ink: '#000000',
    inkSoft: '#1a1a1a',
    inkFaint: '#333333',
    rule: '#000000',
    ruleSoft: '#4d4d4d',
    mark: HIGH_CONTRAST.primary,
    flag: HIGH_CONTRAST.error,
    nightBg: '#000000',
    nightStage: '#000000',
    nightPanel: '#000000',
    nightInk: '#ffffff',
    nightSoft: '#ffffff',
    nightMuted: '#ffffff',
  },
}

/** UI トークンが参照する CSS 変数名。 */
export function surfaceVar(key: keyof StudentSurface): string {
  return `--ui-${key.replace(/[A-Z]/g, (c) => '-' + c.toLowerCase())}`
}

function surfaceCssVars(mode: StudentThemeMode): Record<string, string> {
  return Object.fromEntries(
    Object.entries(STUDENT_SURFACES[mode]).map(([k, v]) => [surfaceVar(k as keyof StudentSurface), v]),
  )
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
    typography: {
      fontFamily: FONT_FAMILY,
      // #1572: 本文の既定サイズ。以前は MUI 既定の14pxのままだった。
      // fontSize だけでなく body1/body2 も明示し、計算係数のズレで意図せず
      // ぶれないようにする（企業ポータル側 company-theme.ts と同じ考え方）。
      fontSize: BODY_FONT_SIZE,
      body1: { fontSize: '1rem', lineHeight: 1.6 },
      body2: { fontSize: '1rem', lineHeight: 1.6 },
      caption: { fontSize: '0.875rem', lineHeight: 1.5 },
      h4: { fontSize: '1.75rem', fontWeight: 700, letterSpacing: '-0.01em' },
      h5: { fontSize: '1.375rem', fontWeight: 700 },
      h6: { fontSize: '1.125rem', fontWeight: 700 },
    },
    shape: { borderRadius: mode === 'high-contrast' ? 4 : 12 },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          ':root': surfaceCssVars(mode),
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
