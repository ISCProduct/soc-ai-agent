/** @jest-environment jsdom */
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { HomeSidebar } from '@/components/home/HomeSidebar'
import {
  HOME_SIDEBAR_ANALYSIS,
  HOME_SIDEBAR_FOOTER,
  HOME_SIDEBAR_GROUPS,
  HOME_SIDEBAR_TOP,
  SIDEBAR_NAV_ITEMS,
} from '@/lib/sidebar-nav'
import type { User } from '@/lib/auth'

let mockPathname = '/'
const push = jest.fn()
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push, replace: jest.fn(), refresh: jest.fn() }),
  usePathname: () => mockPathname,
}))

const user = { user_id: 1, name: '山田 太郎', email: 'student@example.com' } as unknown as User

/** サイドバーはモバイル用とデスクトップ用の2つが描画される。デスクトップ用（後ろ）を見る。 */
function nav() {
  return screen.getAllByRole('navigation', { name: 'メニュー' }).at(-1) as HTMLElement
}

describe('HomeSidebar（ホームの左サイドバー）', () => {
  beforeEach(() => {
    mockPathname = '/'
    push.mockReset()
    global.fetch = jest.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  })

  it('SIDEBAR_NAV_ITEMS の全件が、リンクとして描画される（#403: 適性検査・スカウトの欠落回帰防止）', () => {
    render(<HomeSidebar user={user} onLogout={jest.fn()} />)
    for (const item of SIDEBAR_NAV_ITEMS) {
      const link = within(nav()).getByRole('link', { name: new RegExp(item.label) })
      expect(link).toHaveAttribute('href', item.href)
    }
  })

  it('すべての項目にアイコンがあり、読み上げには出さない（旧サイドバーと同じアイコン）', () => {
    render(<HomeSidebar user={user} onLogout={jest.fn()} />)
    const links = within(nav()).getAllByRole('link')
    expect(links.length).toBeGreaterThanOrEqual(SIDEBAR_NAV_ITEMS.length)
    for (const link of links) {
      const icon = link.querySelector('svg')
      expect(icon).not.toBeNull()
      expect(icon).toHaveAttribute('aria-hidden', 'true')
    }
  })

  it('Figma の並び: ホーム・マッチング結果 / 自己分析 / 準備する / 就活を進める / プロフィール設定', () => {
    render(<HomeSidebar user={user} onLogout={jest.fn()} />)
    const text = nav().textContent ?? ''
    const order = [
      ...HOME_SIDEBAR_TOP.map((i) => i.label),
      '自己分析',
      ...HOME_SIDEBAR_ANALYSIS.map((i) => i.label),
      ...HOME_SIDEBAR_GROUPS.flatMap((g) => [g.heading, ...g.items.map((i) => i.label)]),
      HOME_SIDEBAR_FOOTER.label,
    ]
    let from = 0
    for (const label of order) {
      const at = text.indexOf(label, from)
      expect(at).toBeGreaterThanOrEqual(from)
      from = at + label.length
    }
  })

  it('現在地は、aria-current と太字で示す（ホームのとき、ホームだけ）', () => {
    render(<HomeSidebar user={user} onLogout={jest.fn()} />)
    const current = within(nav()).getAllByRole('link').filter((l) => l.getAttribute('aria-current') === 'page')
    expect(current.map((l) => l.textContent)).toEqual(['ホーム'])
  })

  it('/interview/history にいるとき、面接練習ではなく面接履歴だけが現在地', () => {
    mockPathname = '/interview/history'
    render(<HomeSidebar user={user} onLogout={jest.fn()} />)
    const current = within(nav()).getAllByRole('link').filter((l) => l.getAttribute('aria-current') === 'page')
    expect(current.map((l) => l.textContent)).toEqual(['面接履歴'])
  })

  it('自己分析の進捗は、初期は 判定の進み具合 0% で、チャットの進捗イベントで更新される', () => {
    render(<HomeSidebar user={user} onLogout={jest.fn()} />)
    expect(within(nav()).getByText('判定の進み具合 0%')).toBeInTheDocument()
    expect(within(nav()).getByRole('progressbar', { name: '自己分析の進み具合' })).toHaveAttribute('aria-valuenow', '0')

    act(() => {
      window.dispatchEvent(new CustomEvent('chatProgress', { detail: { messageCount: 7, questionCount: 6, totalQuestions: 15 } }))
    })
    expect(within(nav()).getByText('判定の進み具合 40%')).toBeInTheDocument()
    expect(within(nav()).getByRole('progressbar', { name: '自己分析の進み具合' })).toHaveAttribute('aria-valuenow', '40')
  })

  it('サーバーが判定の進み具合を返したら、質問数から出した割合より優先する', () => {
    render(<HomeSidebar user={user} onLogout={jest.fn()} />)
    act(() => {
      window.dispatchEvent(
        new CustomEvent('chatProgress', { detail: { messageCount: 7, questionCount: 6, totalQuestions: 15, judgmentPercent: 72 } }),
      )
    })
    expect(within(nav()).getByText('判定の進み具合 72%')).toBeInTheDocument()
    expect(within(nav()).getByRole('progressbar', { name: '自己分析の進み具合' })).toHaveAttribute('aria-valuenow', '72')
  })

  it('ログアウトを押すと onLogout を呼ぶ', () => {
    const onLogout = jest.fn()
    render(<HomeSidebar user={user} onLogout={onLogout} />)
    fireEvent.click(screen.getAllByRole('button', { name: 'ログアウト' })[0])
    expect(onLogout).toHaveBeenCalled()
  })

  it('ゲストは選考管理が「ログインが必要です」で、行き先はログイン（既存の挙動）', () => {
    const guest = { ...user, is_guest: true } as unknown as User
    render(<HomeSidebar user={guest} onLogout={jest.fn()} />)
    const link = within(nav()).getByRole('link', { name: /選考管理/ })
    expect(link).toHaveAttribute('href', '/login')
    expect(link).toHaveTextContent('ログインが必要です')
  })

  it('管理者のときだけ、管理者機能のリンクを出す', () => {
    const { unmount } = render(<HomeSidebar user={user} onLogout={jest.fn()} />)
    expect(within(nav()).queryByRole('link', { name: '管理者機能' })).toBeNull()
    unmount()
    render(<HomeSidebar user={{ ...user, is_admin: true } as unknown as User} onLogout={jest.fn()} />)
    expect(within(nav()).getByRole('link', { name: '管理者機能' })).toHaveAttribute('href', '/admin')
  })

  it('マッチング結果は、結果画面（またはチャット）へ router で遷移する', () => {
    render(<HomeSidebar user={user} onLogout={jest.fn()} />)
    fireEvent.click(within(nav()).getByRole('link', { name: 'マッチング結果' }))
    expect(push).toHaveBeenCalledTimes(1)
  })
})
