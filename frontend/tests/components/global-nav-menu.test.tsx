/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import { GlobalNavMenu } from '@/components/global-nav-menu'

let mockPathname = '/interview'

jest.mock('next/navigation', () => ({
  usePathname: () => mockPathname,
}))

describe('GlobalNavMenu', () => {
  afterEach(() => {
    mockPathname = '/interview'
  })

  it('ホーム画面では表示しない（AnalysisSidebar と重複させない）', () => {
    mockPathname = '/'
    const { container } = render(<GlobalNavMenu />)
    expect(container).toBeEmptyDOMElement()
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
    expect(screen.getByRole('link', { name: /企業相関図/ })).toHaveAttribute('href', '/Correlation-diagram')
    expect(screen.getByRole('link', { name: /面接履歴/ })).toHaveAttribute('href', '/interview/history')
  })
})
