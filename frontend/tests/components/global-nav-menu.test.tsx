/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import { GlobalNavMenu } from '@/components/layout/GlobalNavMenu'

let mockPathname = '/interview'

jest.mock('next/navigation', () => ({
  usePathname: () => mockPathname,
}))

const STORED_USER = { user_id: 1, email: 'student@example.com', name: 'テストユーザー', is_guest: false, is_admin: false }

describe('GlobalNavMenu', () => {
  beforeEach(() => {
    // メニューはログイン中の学生にだけ出す（#2222）
    window.localStorage.setItem('user', JSON.stringify(STORED_USER))
  })

  afterEach(() => {
    mockPathname = '/interview'
    window.localStorage.clear()
    window.sessionStorage.clear()
  })

  it('未ログイン（ストレージに user が無い）ではメニューボタンを出さない（#2222）', () => {
    window.localStorage.clear()
    mockPathname = '/privacy'
    const { container } = render(<GlobalNavMenu />)
    expect(container).toBeEmptyDOMElement()
    expect(screen.queryByRole('button', { name: 'メニューを開く' })).not.toBeInTheDocument()
  })

  // #2222 のレビュー: RootLayout の部品は /login でもマウントされたままで、ログインは router.push（SPA の遷移）。
  // 同じマウントのまま、パスが変わったら判定し直すこと
  it('ログインして別の画面へ移ると、同じマウントのままメニューボタンが出る', () => {
    window.localStorage.clear()
    mockPathname = '/privacy'
    const { rerender } = render(<GlobalNavMenu />)
    expect(screen.queryByRole('button', { name: 'メニューを開く' })).not.toBeInTheDocument()

    window.localStorage.setItem('user', JSON.stringify(STORED_USER)) // saveAuth
    mockPathname = '/results' // router.push
    rerender(<GlobalNavMenu />)
    expect(screen.getByRole('button', { name: 'メニューを開く' })).toBeInTheDocument()
  })

  it('ログアウトして別の画面へ移ると、同じマウントのままメニューボタンが消える', () => {
    mockPathname = '/results'
    const { rerender } = render(<GlobalNavMenu />)
    expect(screen.getByRole('button', { name: 'メニューを開く' })).toBeInTheDocument()

    // logout() と同じく、利用者の情報を消す（最初の読み込みで localStorage から sessionStorage へ移っている）
    window.sessionStorage.clear()
    window.localStorage.clear()
    mockPathname = '/privacy'
    rerender(<GlobalNavMenu />)
    expect(screen.queryByRole('button', { name: 'メニューを開く' })).not.toBeInTheDocument()
  })

  it('ログイン済みなら /privacy でもメニューボタンを出す', () => {
    mockPathname = '/privacy'
    render(<GlobalNavMenu />)
    expect(screen.getByRole('button', { name: 'メニューを開く' })).toBeInTheDocument()
  })

  it('ホーム画面では表示しない（HomeSidebar と重複させない）', () => {
    mockPathname = '/'
    const { container } = render(<GlobalNavMenu />)
    expect(container).toBeEmptyDOMElement()
  })

  it('相関図では画面のサイドバー・メニューと重複させない', () => {
    mockPathname = '/correlation-diagram'
    const { container } = render(<GlobalNavMenu />)
    expect(container).toBeEmptyDOMElement()
  })

  it('従来の企業選択画面では共通メニューを表示する', () => {
    mockPathname = '/correlation-diagram'
    render(<GlobalNavMenu correlationSelection />)
    expect(screen.getByRole('button', { name: 'メニューを開く' })).toBeInTheDocument()
  })

  it('ログイン画面では表示しない', () => {
    mockPathname = '/login'
    const { container } = render(<GlobalNavMenu />)
    expect(container).toBeEmptyDOMElement()
  })

  it('/interview ではメニューボタンを表示し、開くとES・スケジュール・選考管理・相関図・面接履歴へのリンクを含む（#1011 受け入れ条件）', async () => {
    mockPathname = '/interview'
    render(<GlobalNavMenu />)

    const trigger = screen.getByRole('button', { name: 'メニューを開く' })
    expect(trigger).toBeInTheDocument()

    fireEvent.click(trigger)

    expect(await screen.findByRole('link', { name: /ESリライト・添削/ })).toHaveAttribute('href', '/es-rewrite')
    expect(screen.getByRole('link', { name: /選考スケジュール/ })).toHaveAttribute('href', '/schedule')
    expect(screen.getByRole('link', { name: /選考管理/ })).toHaveAttribute('href', '/applications')
    expect(screen.getByRole('link', { name: 'お気に入り' })).toHaveAttribute('href', '/favorites')
    expect(screen.getByRole('link', { name: /企業相関図/ })).toHaveAttribute('href', '/correlation-diagram')
    expect(screen.getByRole('link', { name: /面接履歴/ })).toHaveAttribute('href', '/interview/history')
  })
})
