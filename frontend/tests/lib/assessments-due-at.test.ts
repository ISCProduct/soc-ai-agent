/**
 * 受検期限の表示。学生カードはこの文字列だけを期限として見せるので、
 * 年が無いと年明けの期限（1/10）を今年の1/10と読み違える。
 */
import { formatDueAt } from '@/lib/assessments'

describe('formatDueAt', () => {
  it('年・日付・時刻を出す', () => {
    const s = formatDueAt('2027-01-10T14:30:00+09:00')
    expect(s).toContain('2027')
    expect(s).toContain('1/10')
    expect(s).toContain('14:30')
  })

  it('年をまたぐ2つの期限が同じ表示にならない', () => {
    expect(formatDueAt('2026-01-10T14:30:00+09:00')).not.toBe(formatDueAt('2027-01-10T14:30:00+09:00'))
  })

  it('日付として読めない値は空文字', () => {
    expect(formatDueAt('まだ決まっていません')).toBe('')
  })
})
