/** @jest-environment jsdom */
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
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

const resizeObservers: TestResizeObserver[] = []
class TestResizeObserver implements ResizeObserver {
  readonly targets = new Set<Element>()
  constructor(private readonly callback: ResizeObserverCallback) {
    resizeObservers.push(this)
  }
  observe(target: Element) { this.targets.add(target) }
  unobserve(target: Element) { this.targets.delete(target) }
  disconnect = jest.fn(() => this.targets.clear())
  notify() { this.callback([], this) }
}

function resize(target: Element) {
  act(() => resizeObservers.filter((observer) => observer.targets.has(target)).forEach((observer) => observer.notify()))
}

// jsdom はレイアウトを計算しないため、スクロール領域の寸法だけを再現する。
function scrollMetrics(viewport: HTMLElement) {
  const metrics = { clientHeight: 500, scrollHeight: 900 }
  Object.defineProperties(viewport, {
    clientHeight: { configurable: true, get: () => metrics.clientHeight },
    scrollHeight: { configurable: true, get: () => metrics.scrollHeight },
  })
  return metrics
}

/** サイドバーはモバイル用とデスクトップ用の2つが描画される。デスクトップ用（後ろ）を見る。 */
function nav() {
  return screen.getAllByRole('navigation', { name: 'メニュー' }).at(-1) as HTMLElement
}

describe('HomeSidebar（ホームの左サイドバー）', () => {
  const originalResizeObserver = global.ResizeObserver
  const originalMatchMedia = window.matchMedia
  beforeEach(() => {
    mockPathname = '/'
    push.mockReset()
    global.fetch = jest.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
    resizeObservers.length = 0
    global.ResizeObserver = TestResizeObserver
  })

  afterEach(() => {
    global.ResizeObserver = originalResizeObserver
    window.matchMedia = originalMatchMedia
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

  it.each([false, true])('ゲスト名はIDや縦折り返しのラベルを出さず、1行で表示する（mobile=%s）', (mobile) => {
    const guest = { ...user, name: 'Guest_random123456', is_guest: true }
    render(<HomeSidebar user={guest} onLogout={jest.fn()} mobileOpen={mobile} />)
    const account = screen.getByRole('button', { name: 'ログアウト' }).parentElement!.parentElement!
    expect(within(account).getByText('ゲストユーザー')).toHaveStyle({ whiteSpace: 'nowrap' })
    expect(within(account).queryByText(guest.name)).not.toBeInTheDocument()
    expect(within(account).queryByText('ゲスト', { exact: true })).not.toBeInTheDocument()
  })

  it('ログイン済みの表示名は Guest で始まっていても書き換えない', () => {
    render(<HomeSidebar user={{ ...user, name: 'Guestさん', is_guest: false }} onLogout={jest.fn()} />)
    const account = screen.getByRole('button', { name: 'ログアウト' }).parentElement!.parentElement!
    expect(within(account).getByText('Guestさん')).toBeInTheDocument()
    expect(within(account).queryByText('ゲストユーザー')).not.toBeInTheDocument()
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

  it('モバイルの閉じるボタンからメニューを閉じられる', async () => {
    const onMobileClose = jest.fn()
    const { rerender } = render(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen onMobileClose={onMobileClose} />)

    fireEvent.click(screen.getByRole('button', { name: 'メニューを閉じる' }))
    expect(onMobileClose).toHaveBeenCalledTimes(1)

    rerender(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen={false} onMobileClose={onMobileClose} />)
    await waitFor(() => expect(screen.queryByRole('button', { name: 'メニューを閉じる' })).not.toBeInTheDocument())
  })

  it('モバイルの項目を選ぶと、行き先を保ったままメニューを閉じる', () => {
    const onMobileClose = jest.fn()
    render(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen onMobileClose={onMobileClose} />)

    const menu = screen.getByRole('navigation', { name: 'メニュー' })
    const profile = within(menu).getByRole('link', { name: 'プロフィール設定' })
    expect(profile).toHaveAttribute('href', '/profile')
    fireEvent.click(profile)
    expect(onMobileClose).toHaveBeenCalledTimes(1)
  })

  describe.each([false, true])('下端のスクロール案内（mobile=%s）', (mobile) => {
    function viewport() {
      return mobile ? screen.getByRole('navigation', { name: 'メニュー' }) : screen.getByTestId('desktop-menu-scroll')
    }

    it('下に続きがある間だけ矢印を出し、最下部では消え、上に戻ると再表示する', () => {
      render(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen={mobile} />)
      const scroller = viewport()
      scrollMetrics(scroller)
      resize(scroller)

      const fade = screen.getByTestId('menu-scroll-fade')
      expect(fade).toHaveAttribute('aria-hidden', 'true')
      expect(fade).toHaveStyle({ pointerEvents: 'none' })
      const arrow = screen.getByRole('button', { name: '下のメニュー項目を表示' })
      expect(arrow.textContent).toBe('')
      expect(arrow).toHaveStyle({ width: '44px', height: '44px', pointerEvents: 'auto' })
      fireEvent.scroll(scroller, { target: { scrollTop: 200 } })
      expect(screen.getByTestId('menu-scroll-fade')).toBeInTheDocument()
      // 小数点以下の誤差で最下部のフェードが残らない。
      fireEvent.scroll(scroller, { target: { scrollTop: 399.5 } })
      expect(screen.queryByTestId('menu-scroll-fade')).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: '下のメニュー項目を表示' })).not.toBeInTheDocument()
      fireEvent.scroll(scroller, { target: { scrollTop: 0 } })
      expect(screen.getByTestId('menu-scroll-fade')).toBeInTheDocument()
    })

    it.each([false, true])('矢印を押すとメニューを下へ送り、動きを減らす設定にも従う（reduce=%s）', (reduce) => {
      window.matchMedia = jest.fn().mockImplementation((media: string) => ({ matches: reduce, media }))
      const onMobileClose = jest.fn()
      render(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen={mobile} onMobileClose={onMobileClose} />)
      const scroller = viewport()
      scrollMetrics(scroller)
      scroller.scrollBy = jest.fn()
      resize(scroller)
      fireEvent.click(screen.getByRole('button', { name: '下のメニュー項目を表示' }))
      expect(scroller.scrollBy).toHaveBeenCalledWith({ top: 375, behavior: reduce ? 'auto' : 'smooth' })
      expect(onMobileClose).not.toHaveBeenCalled()
    })

    it('操作中の矢印が最下部で消えても、フォーカスを末尾の項目に残す', () => {
      render(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen={mobile} />)
      const scroller = viewport()
      scrollMetrics(scroller)
      resize(scroller)
      const arrow = screen.getByRole('button', { name: '下のメニュー項目を表示' })
      act(() => arrow.focus())
      fireEvent.scroll(scroller, { target: { scrollTop: 400 } })
      expect(arrow).not.toBeInTheDocument()
      expect(within(scroller).getByRole('link', { name: 'プロフィール設定' })).toHaveFocus()
    })

    it('全項目が収まる高さでは消え、画面が狭くなると再表示する', () => {
      render(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen={mobile} />)
      const scroller = viewport()
      const metrics = scrollMetrics(scroller)
      metrics.clientHeight = 900
      resize(scroller)
      expect(screen.queryByTestId('menu-scroll-fade')).not.toBeInTheDocument()
      metrics.clientHeight = 500
      resize(scroller)
      expect(screen.getByTestId('menu-scroll-fade')).toBeInTheDocument()
      metrics.clientHeight = 1000
      fireEvent(window, new Event('resize'))
      expect(screen.queryByTestId('menu-scroll-fade')).not.toBeInTheDocument()
    })

    it('管理者項目の追加などで内容の高さが変わったときも再判定する', () => {
      const { rerender, unmount } = render(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen={mobile} />)
      const scroller = viewport()
      const metrics = scrollMetrics(scroller)
      fireEvent.scroll(scroller, { target: { scrollTop: 400 } })
      expect(screen.queryByTestId('menu-scroll-fade')).not.toBeInTheDocument()

      rerender(<HomeSidebar user={{ ...user, is_admin: true }} onLogout={jest.fn()} mobileOpen={mobile} />)
      expect(within(scroller).getByRole('link', { name: '管理者機能' })).toBeInTheDocument()
      metrics.scrollHeight = 944
      resize(scroller.firstElementChild!)
      expect(screen.getByTestId('menu-scroll-fade')).toBeInTheDocument()
      unmount()
      for (const observer of resizeObservers) expect(observer.disconnect).toHaveBeenCalled()
    })
  })

  it('閉じているモバイルメニューの寸法が変わっても、開き直したときに再判定する', () => {
    const { rerender } = render(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen />)
    const scroller = screen.getByRole('navigation', { name: 'メニュー' })
    const metrics = scrollMetrics(scroller)
    resize(scroller)
    expect(screen.getByTestId('menu-scroll-fade')).toBeInTheDocument()
    rerender(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen={false} />)
    expect(screen.queryByTestId('menu-scroll-fade')).not.toBeInTheDocument()
    metrics.clientHeight = 1000
    rerender(<HomeSidebar user={user} onLogout={jest.fn()} mobileOpen />)
    expect(screen.queryByTestId('menu-scroll-fade')).not.toBeInTheDocument()
  })
})
