import {
  datetimeLocalToISO,
  formatLocalDateKey,
  isoToDatetimeLocal,
} from '@/lib/datetime-local'

describe('datetime-local', () => {
  it('formatLocalDateKey はローカル暦日を返す（UTC の toISOString と一致しないケース）', () => {
    // 2024-01-01 00:30 JST = 2023-12-31 15:30 UTC
    const date = new Date('2024-01-01T00:30:00+09:00')
    expect(formatLocalDateKey(date)).toBe('2024-01-01')
    expect(date.toISOString().slice(0, 10)).toBe('2023-12-31')
  })

  it('isoToDatetimeLocal は UTC ISO をローカル datetime-local 形式へ変換する', () => {
    const iso = '2024-06-15T01:30:00.000Z'
    const local = isoToDatetimeLocal(iso)
    const d = new Date(iso)
    const pad = (n: number) => String(n).padStart(2, '0')
    const expected = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
    expect(local).toBe(expected)
  })

  it('datetimeLocalToISO は datetime-local 文字列を ISO UTC へ変換する', () => {
    const local = '2024-06-15T10:00'
    const iso = datetimeLocalToISO(local)
    expect(new Date(iso).getHours()).toBe(new Date(local).getHours())
    expect(Number.isNaN(new Date(iso).getTime())).toBe(false)
  })
})
