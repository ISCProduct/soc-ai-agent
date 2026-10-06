import { companyFreshness } from '@/lib/admin/company-freshness'

const NOW = new Date('2026-10-01T00:00:00Z')
const iso = (daysAgo: number) => new Date(NOW.getTime() - daysAgo * 86_400_000).toISOString()

/** すべて取得済みで新しい状態を基準にする。 */
const ALL_FRESH = {
  info_fetched_at: iso(1),
  relations_fetched_at: iso(1),
  tech_fetched_at: iso(1),
  jobs_fetched_at: iso(1),
}

describe('companyFreshness', () => {
  it('すべて新しければ取得済みで内訳を出さない', () => {
    expect(companyFreshness(ALL_FRESH, NOW)).toEqual({ freshness: 'fresh', detail: '' })
  })

  it('未取得の系統があれば未取得を代表にする', () => {
    const got = companyFreshness({ ...ALL_FRESH, relations_fetched_at: null }, NOW)
    expect(got).toEqual({ freshness: 'unfetched', detail: '関連企業' })
  })

  it('TTLを超えた系統は期限切れとして出す（求人は7日）', () => {
    const got = companyFreshness({ ...ALL_FRESH, jobs_fetched_at: iso(10) }, NOW)
    expect(got).toEqual({ freshness: 'expired', detail: '求人' })
  })

  it('会社概要は90日を超えてから期限切れになる', () => {
    expect(companyFreshness({ ...ALL_FRESH, info_fetched_at: iso(80) }, NOW).freshness).toBe('due')
    expect(companyFreshness({ ...ALL_FRESH, info_fetched_at: iso(95) }, NOW).freshness).toBe('expired')
  })

  it('未取得と期限切れが同時なら未取得を優先する', () => {
    // 未取得は学生に見せる情報がまだ無い状態なので先に埋める必要がある。
    const got = companyFreshness(
      { ...ALL_FRESH, tech_fetched_at: null, jobs_fetched_at: iso(30) },
      NOW,
    )
    expect(got.freshness).toBe('unfetched')
    expect(got.detail).toBe('技術情報')
  })

  it('期限切れと期限が近いが同時なら期限切れを優先する', () => {
    const got = companyFreshness(
      { ...ALL_FRESH, info_fetched_at: iso(80), jobs_fetched_at: iso(10) },
      NOW,
    )
    expect(got.freshness).toBe('expired')
    expect(got.detail).toBe('求人')
  })

  it('何も取得していない企業は未取得（最初の系統で代表させる）', () => {
    const got = companyFreshness({}, NOW)
    expect(got.freshness).toBe('unfetched')
    expect(got.detail).toBe('会社概要')
  })

  it('不正な日付は未取得として扱う（期限切れに倒さない）', () => {
    // 「古い」と誤認して再取得を促すより、取得できていない扱いが安全。
    const got = companyFreshness({ ...ALL_FRESH, info_fetched_at: 'broken' }, NOW)
    expect(got.freshness).toBe('unfetched')
  })
})
