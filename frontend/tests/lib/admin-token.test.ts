import { adminTokenNeedsRefresh } from '@/lib/auth/admin-token'

describe('adminTokenNeedsRefresh', () => {
  const now = 1_700_000_000

  it('無い・壊れている・8日前は取り直す', () => {
    expect(adminTokenNeedsRefresh(null, now)).toBe(true)
    expect(adminTokenNeedsRefresh('not-a-token', now)).toBe(true)
    expect(adminTokenNeedsRefresh(`${now - 8 * 24 * 60 * 60}.sig`, now)).toBe(true)
  })

  it('発行から1時間なら取り直さない', () => {
    expect(adminTokenNeedsRefresh(`${now - 60 * 60}.sig`, now)).toBe(false)
  })

  it('期限の1時間を切ったら取り直す', () => {
    const issued = now - (7 * 24 * 60 * 60 - 30 * 60)
    expect(adminTokenNeedsRefresh(`${issued}.sig`, now)).toBe(true)
  })
})
