import {
  createStudentMuiTheme,
  FOCUS_WIDTH,
  SCHEDULE_STAGE_COLORS,
  SCHEDULE_STAGE_TEXT,
  SCHEDULE_STAGES,
  STUDENT_SURFACES,
  type StudentThemeMode,
} from '@/lib/student-theme'
import { focusRingSx, scheduleStageColor, UI } from '@/lib/design/tokens'
import { CALL } from '@/app/interview/callTokens'

// #2219: 文字は 4.5:1、枠・フォーカスは 3:1（WCAG 2.x の 1.4.3 / 1.4.11）。色を変えたときに再発しないよう、
// 定義の値から比を計算して固定する。
const MODES: StudentThemeMode[] = ['comfortable', 'high-contrast']

type Rgba = { r: number; g: number; b: number; a: number }

function parseColor(color: string): Rgba {
  const rgba = color.match(/^rgba?\(([^)]+)\)$/)
  if (rgba) {
    const [r, g, b, a = '1'] = rgba[1].split(',').map((v) => v.trim())
    return { r: Number(r), g: Number(g), b: Number(b), a: Number(a) }
  }
  const hex = color.replace('#', '')
  const full = hex.length === 3 ? hex.split('').map((h) => h + h).join('') : hex
  const [r, g, b] = full.match(/../g)!.map((h) => parseInt(h, 16))
  return { r, g, b, a: 1 }
}

/** 半透明の色（MUI の文字色 rgba(0, 0, 0, 0.87) など）を、下の色に重ねた見た目の色にする。 */
function over(fg: string, bg: string): Rgba {
  const f = parseColor(fg)
  const b = parseColor(bg)
  const mix = (x: number, y: number) => x * f.a + y * (1 - f.a)
  return { r: mix(f.r, b.r), g: mix(f.g, b.g), b: mix(f.b, b.b), a: 1 }
}

function luminance({ r, g, b }: Rgba): number {
  const [lr, lg, lb] = [r, g, b].map((v) => {
    const c = v / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * lr + 0.7152 * lg + 0.0722 * lb
}

/** 文字（fg。半透明でもよい）と地（bg）のコントラスト比。 */
function contrast(fg: string, bg: string): number {
  const [hi, lo] = [luminance(over(fg, bg)), luminance(parseColor(bg))].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

function baseline(mode: StudentThemeMode): Record<string, Record<string, string>> {
  return createStudentMuiTheme(mode).components?.MuiCssBaseline?.styleOverrides as Record<string, Record<string, string>>
}

describe('学生画面の色のコントラスト（#2219）', () => {
  it.each(MODES)('%s: 塗りの部品（Chip など）の文字は、塗りに対して 4.5:1 以上になる', (mode) => {
    const { palette } = createStudentMuiTheme(mode)
    // info は MUI の既定（#0288d1）。既定のままだと success・info に白文字が載り、3.42:1・3.86:1 だった。
    for (const key of ['primary', 'secondary', 'success', 'info', 'warning', 'error'] as const) {
      const ratio = contrast(palette[key].contrastText, palette[key].main)
      expect({ key, ok: ratio >= 4.5 }).toEqual({ key, ok: true })
    }
  })

  it.each(MODES)('%s: 予定の段階の札と帯（6種）の白文字は、塗りに対して 4.5:1 以上になる', (mode) => {
    for (const stage of SCHEDULE_STAGES) {
      const ratio = contrast(SCHEDULE_STAGE_TEXT, SCHEDULE_STAGE_COLORS[mode][stage])
      expect({ stage, ok: ratio >= 4.5 }).toEqual({ stage, ok: true })
    }
  })

  it('段階の色は表示設定ごとに CSS 変数で配り、画面はその変数を使う（高コントラストに追従する）', () => {
    for (const mode of MODES) {
      const vars = baseline(mode)[':root']
      for (const stage of SCHEDULE_STAGES) {
        expect(Object.values(vars)).toContain(SCHEDULE_STAGE_COLORS[mode][stage])
      }
    }
    expect(scheduleStageColor('2次面接')).toBe(`var(--ui-stage-2, ${SCHEDULE_STAGE_COLORS.comfortable['2次面接']})`)
  })

  it.each(MODES)('%s: 薄い補助文字（inkFaint）は paper・tint・field のどの地でも 4.5:1 以上になる', (mode) => {
    const s = STUDENT_SURFACES[mode]
    for (const bg of ['paper', 'tint', 'field'] as const) {
      expect({ bg, ok: contrast(s.inkFaint, s[bg]) >= 4.5 }).toEqual({ bg, ok: true })
    }
  })

  it.each(MODES)('%s: チャットの終了確認の見出しの色（flag）は白地で 4.5:1 以上になる', (mode) => {
    expect(contrast(STUDENT_SURFACES[mode].flag, '#ffffff')).toBeGreaterThanOrEqual(4.5)
  })

  it.each(MODES)('%s: 暗い面のフォーカスの枠（nightInk）は、暗い面のどの地でも 3:1 以上になる', (mode) => {
    const s = STUDENT_SURFACES[mode]
    const outline = baseline(mode)['[data-surface="night"] *:focus-visible, [data-surface="night"]:focus-visible']
    expect(outline.outlineColor).toBe(s.nightInk)
    for (const bg of [s.nightBg, s.nightStage, s.nightPanel]) {
      expect(contrast(outline.outlineColor, bg)).toBeGreaterThanOrEqual(3)
    }
    // 面接中の画面の地（callTokens）。枠はボタンの外側（outlineOffset）に出るので、地と面の両方で確かめる
    for (const bg of [CALL.stage, CALL.panel, CALL.control]) {
      expect(contrast(outline.outlineColor, bg)).toBeGreaterThanOrEqual(3)
    }
  })

  it('既定の primary の枠は暗い面では 3:1 に届かない（暗い面に印を付ける理由）', () => {
    const p = createStudentMuiTheme('comfortable').palette.primary.main
    expect(contrast(p, STUDENT_SURFACES.comfortable.nightPanel)).toBeLessThan(3)
  })

  it('フォーカスの枠の太さは表示設定ごとに CSS 変数で配り、個別の枠もその変数を使う（高コントラストで 3px）', () => {
    expect(baseline('comfortable')[':root']['--ui-focus-width']).toBe('2px')
    expect(baseline('high-contrast')[':root']['--ui-focus-width']).toBe(`${FOCUS_WIDTH['high-contrast']}px`)
    expect(FOCUS_WIDTH['high-contrast']).toBe(3)
    expect(focusRingSx()).toEqual({ outline: `var(--ui-focus-width, 2px) solid ${UI.mark}`, outlineOffset: 2 })
  })
})
