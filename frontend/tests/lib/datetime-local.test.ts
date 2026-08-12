import {
  datetimeLocalToISO,
  formatLocalDateKey,
  isoToDatetimeLocal,
} from '@/lib/datetime-local'

describe('datetime-local', () => {
  it('formatLocalDateKey はローカル暦日を返す（UTC の toISOString は使わない）', () => {
    // Date(y, m, d, h, min) はランナーのローカル時刻で解釈されるため TZ 非依存
    const date = new Date(2024, 0, 1, 0, 30)
    expect(formatLocalDateKey(date)).toBe('2024-01-01')
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
