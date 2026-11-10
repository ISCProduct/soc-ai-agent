/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import { PriceBasisBadge } from '@/app/admin/costs/price-basis-badge'

// #2004: 推定・未登録の単価で計算した金額に印を付ける。
describe('PriceBasisBadge', () => {
  it('推定単価には「推定単価」を出す', () => {
    render(<PriceBasisBadge basis="estimated" />)
    expect(screen.getByText('推定単価')).toBeInTheDocument()
  })

  it('単価表に無いモデルには「単価未登録」を出し、gpt-4o 単価で計算していると説明する', () => {
    render(<PriceBasisBadge basis="unknown" />)
    const badge = screen.getByText('単価未登録')
    expect(badge).toHaveAttribute('title', expect.stringContaining('gpt-4o'))
  })

  it.each([['listed'], [undefined], ['']])('公開価格・根拠なし（%p）では何も出さない', (basis) => {
    const { container } = render(<PriceBasisBadge basis={basis} />)
    expect(container.firstChild).toBeNull()
  })
})
