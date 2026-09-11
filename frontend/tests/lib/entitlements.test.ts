import { canUseFeature, type Entitlements } from '@/lib/entitlements'

describe('entitlements', () => {
  const free: Entitlements = {
    plan: 'free',
    features: { matching: true, export: false, company_graph: false },
  }

  // #989: 以前はデモを壊さないという理由でfail-openだったが、UI上一時的に
  // 制限機能が有効表示されてしまう問題があったためfail-closedに変更した。
  it('未取得/取得失敗時はfail-closedで拒否する', () => {
    expect(canUseFeature(null, 'export')).toBe(false)
  })

  it('free の export は拒否する', () => {
    expect(canUseFeature(free, 'export')).toBe(false)
    expect(canUseFeature(free, 'matching')).toBe(true)
  })

  it('featuresにキーが無い場合も拒否する(fail-closed)', () => {
    const missingKey: Entitlements = { plan: 'free', features: {} }
    expect(canUseFeature(missingKey, 'admin')).toBe(false)
  })
})
