/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import ESRewritePage from '@/app/es-rewrite/page'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() }),
  useSearchParams: () => new URLSearchParams('company_name=%E3%82%B5%E3%83%B3%E3%83%97%E3%83%AB%E6%A0%AA%E5%BC%8F%E4%BC%9A%E7%A4%BE'),
}))

describe('ESRewritePage', () => {
  it('company_nameクエリで志望企業名欄がプリフィルされる（結果カードからのES CTA導線）', () => {
    render(<ESRewritePage />)

    expect(screen.getByLabelText(/志望企業名/)).toHaveValue('サンプル株式会社')
  })
})
