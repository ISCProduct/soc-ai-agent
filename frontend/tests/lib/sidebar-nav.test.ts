import {
  BOTTOM_NAV_HEIGHT,
  SIDEBAR_ADMIN_NAV,
  SIDEBAR_NAV_ITEMS,
  shouldShowStudentBottomNav,
  STUDENT_BOTTOM_NAV_ITEMS,
} from '@/lib/sidebar-nav'

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

  it('選考管理・面接履歴（Bottom nav に無い主要機能）へのリンクを含む（#1011）', () => {
    const hrefs = SIDEBAR_NAV_ITEMS.map((i) => i.href)
    expect(hrefs).toContain('/applications')
    expect(hrefs).toContain('/interview/history')
  })

  it('Bottom nav の高さ相当の余白定義を持つ', () => {
    expect(BOTTOM_NAV_HEIGHT).toContain('56px')
  })

  it('管理者ナビは /admin', () => {
    expect(SIDEBAR_ADMIN_NAV.href).toBe('/admin')
  })

  it('モバイル下ナビは5件でログインでは出さない', () => {
    expect(STUDENT_BOTTOM_NAV_ITEMS).toHaveLength(5)
    expect(shouldShowStudentBottomNav('/login')).toBe(false)
    expect(shouldShowStudentBottomNav('/admin/costs')).toBe(false)
    expect(shouldShowStudentBottomNav('/')).toBe(true)
    expect(shouldShowStudentBottomNav('/interview/history')).toBe(true)
  })
})
