/**
 * 受検期限の表示。学生カードはこの文字列だけを期限として見せるので、
 * 年が無いと年明けの期限（1/10）を今年の1/10と読み違える。
 *
 * 実行環境のタイムゾーンに依存しないよう、時刻は「含まれていること」だけを形で見る。
 * 具体の時刻を書くと CI（UTC）とローカル（JST）で結果が変わる。
 * 日時は UTC 正午を使い、どのタイムゾーンでも年がずれないようにする。
 */
import { formatDueAt } from '@/lib/assessments'

describe('formatDueAt', () => {
  it('年・月日・時刻を出す', () => {
    const s = formatDueAt('2027-06-15T12:00:00Z')
    expect(s).toContain('2027')
    expect(s).toMatch(/\d{1,2}\/\d{1,2}/)
    expect(s).toMatch(/\d{1,2}:\d{2}/)
  })

  it('年をまたぐ2つの期限が同じ表示にならない', () => {
    expect(formatDueAt('2026-06-15T12:00:00Z')).not.toBe(formatDueAt('2027-06-15T12:00:00Z'))
  })

  it('日付として読めない値は空文字', () => {
    expect(formatDueAt('まだ決まっていません')).toBe('')
  })
})
