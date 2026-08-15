import {
  createStudentMuiTheme,
  isStudentThemeMode,
  readStudentThemeMode,
  STUDENT_THEME_STORAGE_KEY,
  writeStudentThemeMode,
} from '@/lib/student-theme'

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
})
