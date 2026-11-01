import {
  BOTTOM_NAV_HEIGHT,
  isCurrentNavExclusive,
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
    expect(hrefs).toContain('/correlation-diagram')
    expect(hrefs).toContain('/profile')
  })

  it('選考管理・面接履歴（Bottom nav に無い主要機能）へのリンクを含む（#1011）', () => {
    const hrefs = SIDEBAR_NAV_ITEMS.map((i) => i.href)
    expect(hrefs).toContain('/applications')
    expect(hrefs).toContain('/interview/history')
    expect(hrefs).toContain('/scout')
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
    expect(shouldShowStudentBottomNav('/company-portal/sign-in')).toBe(false)
    expect(shouldShowStudentBottomNav('/admin/costs')).toBe(false)
    expect(shouldShowStudentBottomNav('/')).toBe(true)
    expect(shouldShowStudentBottomNav('/interview/history')).toBe(true)
  })

  describe('isCurrentNavExclusive（#403: /interview と /interview/history の重なり対策）', () => {
    const hrefs = SIDEBAR_NAV_ITEMS.map((i) => i.href)

    it('前方一致が重ならないときは isCurrentNav と同じ', () => {
      expect(isCurrentNavExclusive('/resume', '/resume', hrefs)).toBe(true)
      expect(isCurrentNavExclusive('/resume/edit', '/resume', hrefs)).toBe(true)
      expect(isCurrentNavExclusive('/schedule', '/resume', hrefs)).toBe(false)
    })

    it('/interview/history では /interview 側を現在地にしない', () => {
      expect(isCurrentNavExclusive('/interview/history', '/interview', hrefs)).toBe(false)
      expect(isCurrentNavExclusive('/interview/history', '/interview/history', hrefs)).toBe(true)
    })

    it('/interview 配下（履歴以外）では /interview 側を現在地にする', () => {
      expect(isCurrentNavExclusive('/interview/session/1', '/interview', hrefs)).toBe(true)
      expect(isCurrentNavExclusive('/interview', '/interview', hrefs)).toBe(true)
    })
  })
})
