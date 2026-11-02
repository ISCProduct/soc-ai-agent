import { createTextDiff } from './text-diff'

describe('createTextDiff', () => {
  it('Japanese additions and deletions are represented on the corresponding side', () => {
    const result = createTextDiff('チームで頑張りました。', 'チームで主体的に取り組みました。')

    expect(result.original.map(segment => segment.text).join('')).toBe('チームで頑張りました。')
    expect(result.improved.map(segment => segment.text).join('')).toBe('チームで主体的に取り組みました。')
    expect(result.original.some(segment => segment.kind === 'removed' && segment.text.includes('頑張'))).toBe(true)
    expect(result.improved.some(segment => segment.kind === 'added' && segment.text.includes('主体的'))).toBe(true)
  })

  it('identical text remains unchanged', () => {
    const result = createTextDiff('成果を出しました。', '成果を出しました。')

    expect(result.original).toEqual([
      { text: '成果を出しました。', kind: 'unchanged', start: 0, end: 9 },
    ])
    expect(result.improved).toEqual(result.original)
  })

  it('large inputs fall back to a bounded replacement diff', () => {
    const original = '。'.repeat(2100)
    const improved = '、'.repeat(2100)
    const result = createTextDiff(original, improved)

    expect(result.original.map(segment => segment.text).join('')).toBe(original)
    expect(result.improved.map(segment => segment.text).join('')).toBe(improved)
    expect(result.original.every(segment => segment.kind === 'removed')).toBe(true)
    expect(result.improved.every(segment => segment.kind === 'added')).toBe(true)
  })
})
