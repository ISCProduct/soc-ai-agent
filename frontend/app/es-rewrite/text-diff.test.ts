import { createTextDiff, findUniqueTextRange } from './text-diff'

describe('createTextDiff', () => {
  it('Japanese additions and deletions are represented on the corresponding side', () => {
    const result = createTextDiff('チームで頑張りました。', 'チームで主体的に取り組みました。')

    expect(result.original.map(segment => segment.text).join('')).toBe('チームで頑張りました。')
    expect(result.improved.map(segment => segment.text).join('')).toBe('チームで主体的に取り組みました。')
    expect(result.original.some(segment => segment.kind === 'removed' && segment.text.includes('頑張'))).toBe(true)
    expect(result.improved.some(segment => segment.kind === 'added' && segment.text.includes('主体的'))).toBe(true)
  })

  describe('findUniqueTextRange', () => {
    it('finds excerpts across width, whitespace, combining, and invisible-control differences', () => {
      expect(findUniqueTextRange('前ＡＢＣ\r\nカ\u3099クチカ\u202e後', 'ABC ガクチカ')).toEqual({
        start: 1,
        end: 11,
      })
    })

    it('does not anchor an ambiguous repeated excerpt', () => {
      expect(findUniqueTextRange('重複した引用と重複した引用', '重複した引用')).toBeNull()
    })
  })

  it('identical text remains unchanged', () => {
    const result = createTextDiff('成果を出しました。', '成果を出しました。')

    expect(result.original.map(segment => segment.text).join('')).toBe('成果を出しました。')
    expect(result.improved.map(segment => segment.text).join('')).toBe('成果を出しました。')
    expect(result.original.every(segment => segment.kind === 'unchanged')).toBe(true)
    expect(result.improved.every(segment => segment.kind === 'unchanged')).toBe(true)
    expect(result.usedFallback).toBe(false)
  })

  it('large inputs fall back to a bounded replacement diff', () => {
    const original = '。'.repeat(2100)
    const improved = '、'.repeat(2100)
    const result = createTextDiff(original, improved)

    expect(result.original.map(segment => segment.text).join('')).toBe(original)
    expect(result.improved.map(segment => segment.text).join('')).toBe(improved)
    expect(result.original.every(segment => segment.kind === 'removed')).toBe(true)
    expect(result.improved.every(segment => segment.kind === 'added')).toBe(true)
    expect(result.usedFallback).toBe(true)
  })

  it('tokenizes Japanese text when Intl.Segmenter is unavailable', () => {
    const descriptor = Object.getOwnPropertyDescriptor(Intl, 'Segmenter')
    Object.defineProperty(Intl, 'Segmenter', { configurable: true, value: undefined })

    try {
      const result = createTextDiff('頑張りました。', '主体的に取り組みました。')

      expect(result.original.map(segment => segment.text).join('')).toBe('頑張りました。')
      expect(result.improved.map(segment => segment.text).join('')).toBe('主体的に取り組みました。')
      expect(result.original.some(segment => segment.kind === 'removed')).toBe(true)
      expect(result.improved.some(segment => segment.kind === 'added')).toBe(true)
    } finally {
      if (descriptor) {
        Object.defineProperty(Intl, 'Segmenter', descriptor)
      } else {
        Reflect.deleteProperty(Intl, 'Segmenter')
      }
    }
  })
})
