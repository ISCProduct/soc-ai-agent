/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import { ServiceErrorView } from '@/components/common/ServiceErrorView'
import ServiceErrorPage from '@/app/error/page'
import AppError from '@/app/error'

const mockPush = jest.fn()
const mockReplace = jest.fn()
const mockRefresh = jest.fn()
let mockPathname = '/'
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace, refresh: mockRefresh }),
  usePathname: () => mockPathname,
}))

const mockCaptureException = jest.fn()
jest.mock('@sentry/nextjs', () => ({
  captureException: (e: unknown) => mockCaptureException(e),
}))

// #2213: エラー画面がステータスコードを出さず、/error では元の画面へ戻り（履歴に残さない）、
// 接続の失敗と画面の不具合で文言を分け、h1 と role="alert" と字形を持つこと。
describe('ServiceErrorView', () => {
  beforeEach(() => {
    mockPush.mockClear()
    mockReplace.mockClear()
    mockRefresh.mockClear()
    mockCaptureException.mockClear()
    mockPathname = '/'
    jest.spyOn(console, 'error').mockImplementation(() => {})
  })
  afterEach(() => {
    jest.restoreAllMocks()
  })

  const renderErrorPage = async (params: { code?: string; from?: string | string[] }) => {
    return render(await ServiceErrorPage({ searchParams: Promise.resolve(params) }))
  }

  it('/error?code=502 で「502」を出さない', async () => {
    await renderErrorPage({ code: '502' })
    expect(document.body.textContent).not.toContain('502')
    expect(screen.getByRole('heading', { level: 1, name: '接続できません' })).toBeInTheDocument()
  })

  it('code に任意の文字列が来ても、そのまま出さない', async () => {
    await renderErrorPage({ code: '<script>alert(1)</script>' })
    expect(document.body.textContent).not.toContain('<script>')
    expect(document.body.textContent).not.toContain('alert(1)')
    expect(document.querySelector('script')).toBeNull()
  })

  // #2213 のレビュー: /error のボタンは元の画面へ戻るだけ（元の処理はやり直さない）ので「再試行」とは呼ばない。
  // 一時的なエラー画面は履歴に残さない（replace）
  it('/error の「元の画面に戻る」は、エラー画面を履歴に残さずに、エラーになったページへ戻る', async () => {
    await renderErrorPage({ code: '502', from: '/chat?session=1' })
    expect(screen.queryByRole('button', { name: '再試行' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '元の画面に戻る' }))
    expect(mockReplace).toHaveBeenCalledWith('/chat?session=1')
    expect(mockPush).not.toHaveBeenCalled()
    expect(mockRefresh).not.toHaveBeenCalled()
  })

  it('/error で戻り先が無い・外のサイトのときは、「トップへ」だけを出す', async () => {
    for (const from of [undefined, 'https://evil.example', '//evil.example', ['/a', '/b']]) {
      mockReplace.mockClear()
      const { unmount } = await renderErrorPage({ from })
      expect(screen.queryByRole('button', { name: '元の画面に戻る' })).not.toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'トップへ' }))
      expect(mockReplace).toHaveBeenCalledWith('/')
      unmount()
    }
  })

  it('描画の例外では接続エラーと別の文言を出し、「再試行」で retry を呼ぶ', () => {
    const retry = jest.fn()
    render(<AppError error={new Error('boom')} retry={retry} />)
    expect(screen.getByRole('heading', { level: 1, name: 'うまく表示できません' })).toBeInTheDocument()
    expect(document.body.textContent).not.toContain('接続できません')
    expect(document.body.textContent).not.toContain('boom')
    expect(screen.getByRole('alert')).toHaveTextContent('先生に連絡してください')
    fireEvent.click(screen.getByRole('button', { name: '再試行' }))
    expect(retry).toHaveBeenCalledTimes(1)
    expect(mockPush).not.toHaveBeenCalled()
  })

  // #2213 のレビュー: このエラー境界は管理画面・企業ポータルでも出る
  it.each(['/admin/users', '/company-portal/jobs'])('%s の描画の例外では「先生に連絡」と案内しない', (path) => {
    mockPathname = path
    render(<AppError error={new Error('boom')} retry={() => {}} />)
    expect(screen.getByRole('alert')).not.toHaveTextContent('先生')
    expect(screen.getByRole('alert')).toHaveTextContent('もう一度お試しください')
  })

  it('描画の例外は、画面には出さずにログと監視（Sentry）へ送る', () => {
    const error = new Error('boom')
    render(<AppError error={error} retry={() => {}} />)
    expect(console.error).toHaveBeenCalledWith(error)
    expect(mockCaptureException).toHaveBeenCalledWith(error)
  })

  it('h1 が1つで、role="alert" に本文があり、状態を字形（アイコン）と文字でも示す', () => {
    render(<ServiceErrorView kind="connection" />)
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('サーバーに接続できませんでした')
    expect(alert).toHaveTextContent('接続エラー')
    expect(alert.querySelector('svg')).not.toBeNull()
  })

  it('「トップへ」でトップへ進む（履歴に残さない）', () => {
    render(<ServiceErrorView />)
    fireEvent.click(screen.getByRole('button', { name: 'トップへ' }))
    expect(mockReplace).toHaveBeenCalledWith('/')
  })
})
