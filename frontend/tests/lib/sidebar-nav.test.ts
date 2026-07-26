import { SIDEBAR_ADMIN_NAV, SIDEBAR_NAV_ITEMS } from '@/lib/sidebar-nav'

describe('sidebar-nav', () => {
  it('主要ナビは絶対パスで定義されている', () => {
    expect(SIDEBAR_NAV_ITEMS.length).toBeGreaterThanOrEqual(5)
    for (const item of SIDEBAR_NAV_ITEMS) {
      expect(item.href.startsWith('/')).toBe(true)
      expect(item.label.length).toBeGreaterThan(0)
    }
  })

  it('面接・相関図・プロフィールへのリンクを含む', () => {
    const hrefs = SIDEBAR_NAV_ITEMS.map((i) => i.href)
    expect(hrefs).toContain('/interview')
    expect(hrefs).toContain('/Correlation-diagram')
    expect(hrefs).toContain('/profile')
  })

  it('管理者ナビは /admin', () => {
    expect(SIDEBAR_ADMIN_NAV.href).toBe('/admin')
  })
})
