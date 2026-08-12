import { canUseFeature, type Entitlements } from '@/lib/entitlements'

describe('entitlements', () => {
  const free: Entitlements = {
    plan: 'free',
    features: { matching: true, export: false, company_graph: false },
  }

  it('未取得時はデモを壊さないよう許可する', () => {
    expect(canUseFeature(null, 'export')).toBe(true)
  })

  it('free の export は拒否する', () => {
    expect(canUseFeature(free, 'export')).toBe(false)
    expect(canUseFeature(free, 'matching')).toBe(true)
  })
})
