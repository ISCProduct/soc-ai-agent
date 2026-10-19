import {
  createStudentMuiTheme,
  isStudentThemeMode,
  readStudentThemeMode,
  STUDENT_SURFACES,
  STUDENT_THEME_STORAGE_KEY,
  writeStudentThemeMode,
} from '@/lib/student-theme'
import { UI } from '@/lib/design/tokens'

/** テーマが :root に出す CSS 変数。 */
function rootVars(mode: 'comfortable' | 'high-contrast'): Record<string, string> {
  const baseline = createStudentMuiTheme(mode).components?.MuiCssBaseline?.styleOverrides as Record<string, Record<string, string>>
  return baseline[':root']
}

/** 16進の相対輝度（WCAG）。 */
function luminance(hex: string): number {
  const c = hex.replace('#', '').match(/../g)!.map((h) => parseInt(h, 16) / 255)
  const [r, g, b] = c.map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}
function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

function memoryStorage(initial: Record<string, string> = {}) {
  const data = { ...initial }
  return {
    getItem: (k: string) => (k in data ? data[k] : null),
    setItem: (k: string, v: string) => {
      data[k] = v
    },
    snapshot: () => data,
  }
}

describe('student-theme', () => {
  it('未知の値は comfortable に倒す', () => {
    expect(isStudentThemeMode('dark')).toBe(false)
    expect(readStudentThemeMode(memoryStorage())).toBe('comfortable')
    expect(readStudentThemeMode(memoryStorage({ [STUDENT_THEME_STORAGE_KEY]: 'nope' }))).toBe(
      'comfortable',
    )
  })

  it('保存したモードを読み戻す', () => {
    const s = memoryStorage()
    writeStudentThemeMode('high-contrast', s)
    expect(s.snapshot()[STUDENT_THEME_STORAGE_KEY]).toBe('high-contrast')
    expect(readStudentThemeMode(s)).toBe('high-contrast')
  })

  it('comfortable の primary は色覚セーフ青', () => {
    const theme = createStudentMuiTheme('comfortable')
    expect(theme.palette.primary.main.toUpperCase()).toBe('#0072B2')
    expect(theme.palette.mode).toBe('light')
  })

  it('high-contrast はより濃い primary と太いフォーカス', () => {
    const theme = createStudentMuiTheme('high-contrast')
    expect(theme.palette.primary.main.toUpperCase()).toBe('#0B3D91')
    expect(theme.palette.background.default).toBe('#ffffff')
    expect(JSON.stringify(theme.components?.MuiCssBaseline)).toContain('3px solid')
  })

  it('画面 primary にブランド橙を使わない', () => {
    for (const mode of ['comfortable', 'high-contrast'] as const) {
      expect(createStudentMuiTheme(mode).palette.primary.main.toLowerCase()).not.toBe('#ec5b13')
    }
  })

  // #1572: 地の色の出どころはテーマ1箇所。/results だけ別の色になっていた。
  it('地の色はテーマの background と同じ値を CSS 変数で配る', () => {
    for (const mode of ['comfortable', 'high-contrast'] as const) {
      const vars = rootVars(mode)
      expect(vars['--ui-paper']).toBe(createStudentMuiTheme(mode).palette.background.default)
      expect(vars['--ui-night-bg']).toBe(STUDENT_SURFACES[mode].nightBg)
    }
    expect(UI.paper).toBe('var(--ui-paper, #f4f7fb)')
  })

  // #1571: 直書きをやめた画面が高コントラストに追従する前提。値が同じだと切り替えても何も変わらない。
  it('high-contrast は文字と罫線のコントラストが上がる', () => {
    const c = STUDENT_SURFACES.comfortable
    const h = STUDENT_SURFACES['high-contrast']
    for (const key of ['ink', 'inkSoft', 'inkFaint', 'rule'] as const) {
      expect(contrast(h[key], h.field)).toBeGreaterThan(contrast(c[key], c.field))
    }
    expect(contrast(h.nightMuted, h.nightBg)).toBeGreaterThan(contrast(c.nightMuted, c.nightBg))
  })

  // #1572: 本文サイズは MUI 既定の14pxのまま放置されていた。16px相当に揃える。
  it('本文サイズは16px相当である', () => {
    for (const mode of ['comfortable', 'high-contrast'] as const) {
      const typography = createStudentMuiTheme(mode).typography
      expect(typography.fontSize).toBe(16)
      expect(typography.body1.fontSize).toBe('1rem')
      expect(typography.body2.fontSize).toBe('1rem')
    }
  })

  // fontSize(14→16)はMUI内部でcoef=fontSize/14として全variantに掛かるため、
  // 明示していないvariantまで意図せず拡大・縮小しないことを確認する（レビュー指摘）。
  it('fontSizeの変更がh1-h3・subtitle・button・overlineの見た目を変えない', () => {
    for (const mode of ['comfortable', 'high-contrast'] as const) {
      const typography = createStudentMuiTheme(mode).typography
      expect(typography.h1.fontSize).toBe('6rem')
      expect(typography.h2.fontSize).toBe('3.75rem')
      expect(typography.h3.fontSize).toBe('3rem')
      expect(typography.subtitle1.fontSize).toBe('1rem')
      expect(typography.subtitle2.fontSize).toBe('0.875rem')
      expect(typography.button.fontSize).toBe('0.875rem')
      expect(typography.overline.fontSize).toBe('0.75rem')
    }
  })

  it('本文に使う文字色は既定でも白地で 4.5:1 を満たす', () => {
    for (const mode of ['comfortable', 'high-contrast'] as const) {
      const s = STUDENT_SURFACES[mode]
      for (const key of ['ink', 'inkSoft', 'inkFaint'] as const) {
        expect(contrast(s[key], s.field)).toBeGreaterThanOrEqual(4.5)
      }
    }
  })
})
