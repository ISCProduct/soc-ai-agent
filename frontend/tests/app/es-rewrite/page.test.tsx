/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ESRewritePage from '@/app/es-rewrite/page-content'

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

describe('ESRewritePage エラー表示 (#1015)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('プロキシがJSONエラーを返した場合、日本語の短いメッセージを表示する（生JSONは見せない）', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      json: async () => ({ error: 'ログインの有効期限が切れました。再度ログインしてください。', status: 401 }),
    })

    render(<ESRewritePage />)

    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'AIでリライトする' }))

    expect(
      await screen.findByText('ログインの有効期限が切れました。再度ログインしてください。'),
    ).toBeInTheDocument()
    expect(screen.queryByText(/^\{.*"error"/)).not.toBeInTheDocument()
  })

  it('プロキシがJSONを返さない場合はフォールバックの日本語文言を表示する', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      json: async () => {
        throw new Error('not json')
      },
    })

    render(<ESRewritePage />)

    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'AIでリライトする' }))

    await waitFor(() => {
      expect(screen.getByText('リライトに失敗しました。再試行してください。')).toBeInTheDocument()
    })
  })
})
