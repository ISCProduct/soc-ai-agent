import { pickNextActions, UPCOMING_DAYS } from '@/lib/next-actions'

const now = new Date('2026-10-11T00:00:00Z')
const at = (days: number) => new Date(now.getTime() + days * 86400000).toISOString()

describe('pickNextActions', () => {
  it('直近の予定を日時の近い順に先頭へ置く', () => {
    const got = pickNextActions(
      [
        { company_name: 'B社', stage: '二次面接', title: '', scheduled_at: at(3) },
        { company_name: 'A社', stage: '一次面接', title: '', scheduled_at: at(1) },
      ],
      [{ company_name: 'C社', status: 'offered' }],
      now,
    )
    expect(got.map((a) => a.text.split(' ').slice(-2).join(' '))).toEqual(['A社 一次面接', 'B社 二次面接', expect.any(String)])
    expect(got[2].text).toContain('C社 から内定')
  })

  it('過去の予定と期間外の予定は出さない', () => {
    const got = pickNextActions(
      [
        { company_name: '過去社', stage: '面接', title: '', scheduled_at: at(-1) },
        { company_name: '先の社', stage: '面接', title: '', scheduled_at: at(UPCOMING_DAYS + 1) },
        { company_name: '壊れた社', stage: '面接', title: '', scheduled_at: 'not-a-date' },
      ],
      [],
      now,
    )
    expect(got).toEqual([])
  })

  it('応募するか未決の企業はまとめて1件にする', () => {
    const got = pickNextActions([], [
      { company_name: 'X社', status: 'not_applied' },
      { company_name: 'Y社', status: 'not_applied' },
      { company_name: 'Z社', status: 'applied' },
    ], now)
    expect(got).toHaveLength(1)
    expect(got[0].text).toBe('応募するか決めていない企業が2社あります')
    expect(got[0].href).toBe('/applications')
  })

  it('題名があれば段階より題名を出す', () => {
    const [a] = pickNextActions([{ company_name: 'A社', stage: '面接', title: '最終面接（役員）', scheduled_at: at(2) }], [], now)
    expect(a.text).toContain('A社 最終面接（役員）')
  })
})
