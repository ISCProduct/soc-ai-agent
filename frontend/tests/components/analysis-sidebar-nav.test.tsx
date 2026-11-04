/**
 * @jest-environment jsdom
 */

/**
 * AnalysisSidebar（ホーム画面専用サイドバー）のナビ項目の網羅性（#403）。
 *
 * 以前は SIDEBAR_NAV_ITEMS（lib/sidebar-nav.ts）に項目を追加しても、AnalysisSidebar 側は
 * 個別ハードコードのボタン列のため反映されず、「適性検査」「スカウト」への導線が無い状態に
 * なっていた。この回帰を検知するため、SIDEBAR_NAV_ITEMS 全件分のボタンが実際に描画される
 * ことを確認する。
 *
 * 現在地ハイライトは追加しない: AnalysisSidebar は `frontend/app/page-content.tsx`（ホーム画面
 * `/`）でしか描画されず、SIDEBAR_NAV_ITEMS に `/` はひとつも含まれないため、現在地判定は
 * 常に「どれも現在地ではない」にしかならない。現在地ハイライトは GlobalNavMenu（ホーム以外の
 * 全ページで表示される共通メニュー）側で既に実装・テスト済み（global-nav-menu.test.tsx）。
 */
import { render, screen } from '@testing-library/react'
import { AnalysisSidebar } from '@/components/AnalysisSidebar'
import { SIDEBAR_NAV_ITEMS } from '@/lib/sidebar-nav'
import type { User } from '@/lib/auth'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn(), refresh: jest.fn() }),
}))

const user = { user_id: 1, name: '山田太郎', email: 'student@example.com' } as unknown as User

describe('AnalysisSidebar のナビ項目', () => {
  beforeEach(() => {
    global.fetch = jest.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  })

  it('SIDEBAR_NAV_ITEMS の全件ぶんボタンがある（#403: 適性検査・スカウトの欠落回帰防止）', () => {
    render(<AnalysisSidebar user={user} onLogout={jest.fn()} />)
    for (const item of SIDEBAR_NAV_ITEMS) {
      expect(screen.getAllByRole('button', { name: new RegExp(item.label) }).length).toBeGreaterThan(0)
    }
  })

  it('ゲストは選考管理にTooltipで「ログインが必要です」と出る（既存挙動の回帰確認）', () => {
    const guest = { ...user, is_guest: true } as unknown as User
    render(<AnalysisSidebar user={guest} onLogout={jest.fn()} />)
    expect(screen.getAllByText('ログインが必要です').length).toBeGreaterThan(0)
  })
})
