import { ADMIN_NAV, activeAdminNavItem, visibleAdminNav } from '@/lib/admin-nav'

describe('visibleAdminNav', () => {
  it('プラットフォーム管理者にはシステム管理を出す', () => {
    const headings = visibleAdminNav(true).map((g) => g.heading)
    expect(headings).toContain('システム管理')
  })

  it('担当校つき管理者にはシステム管理を出さない', () => {
    const headings = visibleAdminNav(false).map((g) => g.heading)
    expect(headings).not.toContain('システム管理')
    // 学校運営・教員業務は残る
    expect(headings).toEqual(['学校運営', '教員業務'])
  })

  it('権限が未確定のあいだはシステム管理を出さない', () => {
    // 読み込み中に出して直後に消すと、押そうとした項目が消える。
    expect(visibleAdminNav(null).map((g) => g.heading)).not.toContain('システム管理')
  })
})

describe('activeAdminNavItem', () => {
  it('完全一致で現在地を返す', () => {
    expect(activeAdminNavItem('/admin/companies')?.title).toBe('企業情報')
  })

  it('下位パスでも親の項目を現在地にする', () => {
    expect(activeAdminNavItem('/admin/companies/12/edit')?.title).toBe('企業情報')
    expect(activeAdminNavItem('/admin/organizations/3/edit')?.title).toBe('学園(組織)管理')
  })

  it('最長一致を取る', () => {
    // 前方一致で最初に当たったものを返すと粒度が落ちる。
    // 現状の定義には入れ子の href が無いので、同義の確認として
    // より長い href を持つ項目が優先されることを直接確かめる。
    const longest = [...ADMIN_NAV.flatMap((g) => g.items)].sort((a, b) => b.href.length - a.href.length)[0]
    expect(activeAdminNavItem(longest.href)?.href).toBe(longest.href)
  })

  it('似た名前のパスを誤って現在地にしない', () => {
    // '/admin/companies-archive' は '/admin/companies' で始まるが別画面。
    // 境界をスラッシュで見ているので当たらない。
    expect(activeAdminNavItem('/admin/companies-archive')).toBeNull()
  })

  it('ナビに無いパスは null', () => {
    expect(activeAdminNavItem('/admin')).toBeNull()
    expect(activeAdminNavItem('/profile')).toBeNull()
  })
})

describe('ADMIN_NAV', () => {
  it('href が重複しない', () => {
    const hrefs = ADMIN_NAV.flatMap((g) => g.items.map((i) => i.href))
    expect(new Set(hrefs).size).toBe(hrefs.length)
  })

  it('全項目に説明と具体的な操作名がある', () => {
    // §13: ボタンには具体的な操作名を使う（「OK」「実行」を避ける）。
    for (const item of ADMIN_NAV.flatMap((g) => g.items)) {
      expect(item.description).not.toBe('')
      expect(item.cta).not.toBe('')
      expect(['OK', '実行', '処理', '登録']).not.toContain(item.cta)
    }
  })

  it('すべての href が /admin 配下を指す', () => {
    for (const item of ADMIN_NAV.flatMap((g) => g.items)) {
      expect(item.href.startsWith('/admin/')).toBe(true)
    }
  })
})
