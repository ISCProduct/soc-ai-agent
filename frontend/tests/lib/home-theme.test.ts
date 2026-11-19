import { HOME_COLORS } from '@/lib/home-theme'
import { familyNameOf, greetingFor } from '@/lib/home-greeting'

/** WCAG 2.x の相対輝度とコントラスト比。 */
function luminance(hex: string): number {
  const h = hex.length === 4 ? `#${hex[1]}${hex[1]}${hex[2]}${hex[2]}${hex[3]}${hex[3]}` : hex
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = parseInt(h.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}
function contrast(fg: string, bg: string): number {
  const [a, b] = [luminance(fg), luminance(bg)].sort((x, y) => y - x)
  return (a + 0.05) / (b + 0.05)
}

describe('ホームの配色（Figma の値）の、文字と背景のコントラスト', () => {
  // 文字は 4.5:1 以上（PRD の非機能要件）。Figma の色をそのまま使うので、実測で固定する。
  const pairs: [string, string, string][] = [
    ['本文 / 白', HOME_COLORS.ink, HOME_COLORS.white],
    ['本文 / 画面の地', HOME_COLORS.ink, HOME_COLORS.page],
    ['補助の文字 / 白', HOME_COLORS.soft, HOME_COLORS.white],
    ['補助の文字 / 画面の地', HOME_COLORS.soft, HOME_COLORS.page],
    ['リンク・ラベル / 白', HOME_COLORS.link, HOME_COLORS.white],
    ['ナビの文字 / 白', HOME_COLORS.navInk, HOME_COLORS.white],
    ['ナビの文字 / 画面の地（ホバー）', HOME_COLORS.navInk, HOME_COLORS.page],
    ['現在地の文字 / 現在地の背景', HOME_COLORS.primary, HOME_COLORS.activeBg],
    ['ボタンの白文字 / ボタン', HOME_COLORS.white, HOME_COLORS.primary],
    ['案内の文字 / 案内の背景', HOME_COLORS.noticeInk, HOME_COLORS.noticeBg],
    ['アウトラインボタンの文字 / 白', HOME_COLORS.link, HOME_COLORS.white],
  ]
  it.each(pairs)('%s は 4.5:1 以上', (_name, fg, bg) => {
    expect(contrast(fg, bg)).toBeGreaterThanOrEqual(4.5)
  })

  it('入力欄の枠は、白との差が 3:1 以上（非文字の UI）', () => {
    expect(contrast(HOME_COLORS.inputBorder, HOME_COLORS.white)).toBeGreaterThanOrEqual(3)
  })
})

describe('挨拶', () => {
  it('姓だけを使う（空白は全角も）', () => {
    expect(familyNameOf('山田 太郎')).toBe('山田')
    expect(familyNameOf('山田　太郎')).toBe('山田')
    expect(greetingFor('山田 太郎')).toBe('山田さん、こんにちは')
  })
  it('空白が無い名前はそのまま、名前が無いときは名前なしの挨拶', () => {
    expect(familyNameOf('山田太郎')).toBe('山田太郎')
    expect(greetingFor('')).toBe('こんにちは')
    expect(greetingFor(undefined)).toBe('こんにちは')
    expect(greetingFor('  ')).toBe('こんにちは')
  })
})
