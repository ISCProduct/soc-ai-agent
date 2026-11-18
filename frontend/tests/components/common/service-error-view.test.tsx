/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import { ServiceErrorView } from '@/components/common/ServiceErrorView'
import ServiceErrorPage from '@/app/error/page'
import AppError from '@/app/error'

const mockPush = jest.fn()
const mockRefresh = jest.fn()
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, refresh: mockRefresh }),
}))

// #2213: エラー画面がステータスコードを出さず、「再試行」が元の処理をやり直し、
// 接続の失敗と画面の不具合で文言を分け、h1 と role="alert" と字形を持つこと。
describe('ServiceErrorView', () => {
  beforeEach(() => {
    mockPush.mockClear()
    mockRefresh.mockClear()
  })

  const renderErrorPage = async (params: { code?: string; from?: string | string[] }) => {
    render(await ServiceErrorPage({ searchParams: Promise.resolve(params) }))
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

  it('/error の「再試行」は、/error を読み直さず、エラーになったページへ進む', async () => {
    await renderErrorPage({ code: '502', from: '/chat?session=1' })
    fireEvent.click(screen.getByRole('button', { name: '再試行' }))
    expect(mockPush).toHaveBeenCalledWith('/chat?session=1')
    expect(mockRefresh).not.toHaveBeenCalled()
  })

  it('/error の「再試行」は、戻り先が無い・外のサイトのときはトップへ進む', async () => {
    for (const from of [undefined, 'https://evil.example', '//evil.example', ['/a', '/b']]) {
      mockPush.mockClear()
      const { unmount } = render(await ServiceErrorPage({ searchParams: Promise.resolve({ from }) }))
      fireEvent.click(screen.getByRole('button', { name: '再試行' }))
      expect(mockPush).toHaveBeenCalledWith('/')
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

  it('h1 が1つで、role="alert" に本文があり、状態を字形（アイコン）と文字でも示す', () => {
    render(<ServiceErrorView kind="connection" />)
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('サーバーに接続できませんでした')
    expect(alert).toHaveTextContent('接続エラー')
    expect(alert.querySelector('svg')).not.toBeNull()
  })

  it('「トップへ」でトップへ進む', () => {
    render(<ServiceErrorView />)
    fireEvent.click(screen.getByRole('button', { name: 'トップへ' }))
    expect(mockPush).toHaveBeenCalledWith('/')
  })
})
