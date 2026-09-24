/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import RegisterConfirmPage from '@/app/register/confirm/page-content'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() }),
  useSearchParams: () => new URLSearchParams('token=dummy-token'),
}))

jest.mock('@/lib/auth', () => ({
  authService: {
    verifyRegistration: jest.fn().mockResolvedValue({ email: 'student@example.com' }),
    register: jest.fn(),
    saveAuth: jest.fn(),
  },
}))

describe('RegisterConfirmPage', () => {
  // 新規アカウントのパスワード設定画面。autoComplete が無いとブラウザ／パスワード
  // マネージャーが「新しいパスワード」欄と認識せず、生成・確認欄への反映・保存が
  // 働かない（#1479）。/reset-password と同じ new-password を両欄に付ける。
  it('パスワード欄と確認欄の両方に autocomplete="new-password" が付く', async () => {
    render(<RegisterConfirmPage />)

    // トークン検証が解決するまでフォームは出ない。
    const password = await screen.findByLabelText('パスワード *')
    const confirmPassword = screen.getByLabelText('パスワード（確認） *')

    expect(password).toHaveAttribute('autocomplete', 'new-password')
    expect(confirmPassword).toHaveAttribute('autocomplete', 'new-password')
  })

  it('ページ見出しは h1', async () => {
    render(<RegisterConfirmPage />)

    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('会員登録の完了')
  })
})
