import {
  MIN_MEASURED_FOR_WEAKEST,
  UPCOMING_DAYS,
  WEAK_SCORE_BELOW,
  findWeakestCategory,
  pickGrowthActions,
  pickNextActions,
} from '@/lib/next-actions'

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

// SOCAIAGENT-614: 診断スコアに応じた改善の提案。
describe('診断・面接の改善の提案', () => {
  const scores = (pairs: Record<string, number>) => Object.entries(pairs).map(([category, score]) => ({ category, score }))
  const base = { 技術志向: 70, チームワーク志向: 65, リーダーシップ志向: 60, コミュニケーション力: 55 }

  it('いちばん低い練習の対象のカテゴリを選び、面接か ES の練習へつなぐ', () => {
    const got = pickGrowthActions({ diagnosis: scores({ ...base, コミュニケーション力: 40, 細部志向: 45 }), interviewCount: 1 })
    expect(got).toHaveLength(1)
    expect(got[0].text).toBe('「コミュニケーション力」が 40 と、ほかより低めです。面接で、結論から短く話す練習をしてみましょう')
    expect(got[0].href).toBe('/interview')

    const es = pickGrowthActions({ diagnosis: scores({ ...base, 技術志向: 30 }), interviewCount: 1 })
    expect(es[0].href).toBe('/es-rewrite')
    expect(es[0].actionLabel).toBe('ES で練習する')
  })

  it('安定志向・ワークライフバランスは好みなので、低くても弱みとして挙げない', () => {
    expect(findWeakestCategory(scores({ ...base, 安定志向: 5, ワークライフバランス: 10 }))).toBeNull()
  })

  it('低めの境界（50未満）と、測れた数の下限（4）', () => {
    expect(findWeakestCategory(scores({ ...base, コミュニケーション力: WEAK_SCORE_BELOW }))).toBeNull()
    expect(findWeakestCategory(scores({ ...base, コミュニケーション力: WEAK_SCORE_BELOW - 1 }))?.category).toBe('コミュニケーション力')
    expect(MIN_MEASURED_FOR_WEAKEST).toBe(4)
    expect(findWeakestCategory(scores({ 技術志向: 10, 成長志向: 20, 細部志向: 30 }))).toBeNull()
  })

  it('同じ点なら決まった並びで選ぶ', () => {
    expect(findWeakestCategory(scores({ ...base, 技術志向: 30, コミュニケーション力: 30 }))?.category).toBe('コミュニケーション力')
  })

  it('同じ点の傾向がほかにもあるときは「ほかより低め」と書かない', () => {
    const tied = pickGrowthActions({ diagnosis: scores({ ...base, 技術志向: 30, コミュニケーション力: 30 }), interviewCount: 1 })
    expect(tied[0].text).toBe('「コミュニケーション力」が 30 と低めです。面接で、結論から短く話す練習をしてみましょう')
    expect(findWeakestCategory(scores({ ...base, コミュニケーション力: 30 }))?.tied).toBe(false)
  })

  it('診断・面接がまだなら始めることを勧める。診断はホームのチャットで進めるのでボタンを出さない', () => {
    const got = pickGrowthActions({ diagnosis: [], interviewCount: 0 })
    expect(got.map((a) => a.key)).toEqual(['diagnosis-start', 'interview-start'])
    expect(got[0].href).toBeUndefined()
    expect(got[1].href).toBe('/interview')
  })

  it('期限のある予定・内定の後ろ、応募の未決の前に置く。状況が取れなければ出さない', () => {
    const growth = { diagnosis: [], interviewCount: 1 }
    const got = pickNextActions(
      [{ company_name: 'A社', stage: '一次面接', title: '', scheduled_at: at(1) }],
      [{ company_name: 'B社', status: 'offered' }, { company_name: 'C社', status: 'not_applied' }],
      now,
      growth,
    )
    expect(got.map((a) => a.key)).toEqual(['schedule-0', 'offered-B社', 'diagnosis-start', 'not-applied'])
    expect(pickNextActions([], [], now, null)).toEqual([])
  })
})
