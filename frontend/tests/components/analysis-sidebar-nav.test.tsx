/**
 * @jest-environment jsdom
 */

/**
 * AnalysisSidebar（PC常設サイドバー）のナビ項目の網羅性と現在地表示（#403）。
 *
 * 以前は SIDEBAR_NAV_ITEMS（lib/sidebar-nav.ts）に項目を追加しても、AnalysisSidebar 側は
 * 個別ハードコードのボタン列のため反映されず、「適性検査」「スカウト」への導線が無い状態に
 * なっていた。この回帰を検知するため、SIDEBAR_NAV_ITEMS 全件分のボタンが実際に描画される
 * ことを確認する。現在地ハイライト（GlobalNavMenu / StudentBottomNav と同じ aria-current）も
 * ここで押さえる。
 */
import { render, screen } from '@testing-library/react'
import { AnalysisSidebar } from '@/components/AnalysisSidebar'
import { SIDEBAR_NAV_ITEMS } from '@/lib/sidebar-nav'
import type { User } from '@/lib/auth'

let mockPathname = '/'
const push = jest.fn()

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push, replace: jest.fn(), refresh: jest.fn() }),
  usePathname: () => mockPathname,
}))

const user = { user_id: 1, name: '山田太郎', email: 'student@example.com' } as unknown as User

describe('AnalysisSidebar のナビ項目', () => {
  beforeEach(() => {
    mockPathname = '/'
    push.mockClear()
    global.fetch = jest.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  })

  it('SIDEBAR_NAV_ITEMS の全件ぶんボタンがある（#403: 適性検査・スカウトの欠落回帰防止）', () => {
    render(<AnalysisSidebar user={user} onLogout={jest.fn()} />)
    for (const item of SIDEBAR_NAV_ITEMS) {
      expect(screen.getAllByRole('button', { name: new RegExp(item.label) }).length).toBeGreaterThan(0)
    }
  })

  it('今いる画面のボタンに aria-current="page" が付く', () => {
    mockPathname = '/schedule'
    render(<AnalysisSidebar user={user} onLogout={jest.fn()} />)

    const current = screen.getAllByRole('button', { name: /選考スケジュール/ })
    expect(current.length).toBeGreaterThan(0)
    for (const el of current) expect(el).toHaveAttribute('aria-current', 'page')

    const others = screen.getAllByRole('button', { name: /企業相関図/ })
    for (const el of others) expect(el).not.toHaveAttribute('aria-current')
  })

  it('/interview/history では「面接練習」ではなく「面接履歴」側が現在地になる（前方一致の重なり対策）', () => {
    mockPathname = '/interview/history'
    render(<AnalysisSidebar user={user} onLogout={jest.fn()} />)

    for (const el of screen.getAllByRole('button', { name: /面接履歴/ })) {
      expect(el).toHaveAttribute('aria-current', 'page')
    }
    for (const el of screen.getAllByRole('button', { name: /面接練習/ })) {
      expect(el).not.toHaveAttribute('aria-current')
    }
  })
})
