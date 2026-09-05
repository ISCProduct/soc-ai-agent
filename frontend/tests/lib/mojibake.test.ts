import { fixMojibake } from '@/lib/mojibake'

describe('fixMojibake', () => {
  // #1068: 単純な文字含有チェック(/[Ãå][^\s]/)は正当な氏名を誤って壊していた。
  // 往復変換で不自然な文字が実際に減る場合のみ採用する。
  it('leaves valid Nordic/French names untouched', () => {
    expect(fixMojibake('Åke Andersson')).toBe('Åke Andersson')
    expect(fixMojibake('François Dupont')).toBe('François Dupont')
    expect(fixMojibake('José García')).toBe('José García')
  })

  it('leaves ASCII-only names untouched', () => {
    expect(fixMojibake('John Smith')).toBe('John Smith')
  })

  it('leaves empty string untouched', () => {
    expect(fixMojibake('')).toBe('')
  })

  it('repairs actual mojibake (UTF-8 bytes misread as Latin-1)', () => {
    // "François" のUTF-8バイト列をLatin-1のcode pointとして誤解釈した文字列を生成
    const utf8Bytes = Buffer.from('François', 'utf-8')
    const misread = Array.from(utf8Bytes)
      .map((b) => String.fromCharCode(b))
      .join('')
    expect(fixMojibake(misread)).toBe('François')
  })

  it('does not throw and falls back to the original string on invalid byte sequences', () => {
    // decodeURIComponent(escape(s)) が URIError を投げうる入力でも例外を漏らさない
    const input = 'Ã\uD800' // 不正なサロゲート単体を含む
    expect(() => fixMojibake(input)).not.toThrow()
  })
})
