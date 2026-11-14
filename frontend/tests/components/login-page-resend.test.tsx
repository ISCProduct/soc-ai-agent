/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { LoginPage } from '@/components/auth/LoginPage'

jest.mock('@/components/layout/StudentThemeToggle', () => ({ StudentThemeToggle: () => null }))

function jsonResponse(status: number, body: unknown) {
  return { ok: status >= 200 && status < 300, status, json: async () => body }
}

async function loginAsUnverified(fetchMock: jest.Mock) {
  global.fetch = fetchMock as unknown as typeof fetch
  render(<LoginPage onAuthSuccess={jest.fn()} />)
  fireEvent.change(screen.getByLabelText(/メールアドレス/), { target: { value: 'a@example.com' } })
  fireEvent.change(screen.getByLabelText(/パスワード/), { target: { value: 'password123' } })
  fireEvent.click(screen.getByRole('button', { name: 'ログイン' }))
  return screen.findByRole('button', { name: '認証メールを再送' })
}

describe('LoginPage の認証メール再送', () => {
  afterEach(() => jest.restoreAllMocks())

  it('未認証でログインできないときだけ再送ボタンが出て、押すと再送APIにメールアドレスを送る', async () => {
    const fetchMock = jest
      .fn()
      .mockResolvedValueOnce(jsonResponse(403, { error: 'email_not_verified' }))
      .mockResolvedValueOnce(jsonResponse(200, { message: 'ok' }))
    const resend = await loginAsUnverified(fetchMock)

    fireEvent.click(resend)

    await waitFor(() => expect(screen.getByText(/該当するアカウントがある場合は/)).toBeInTheDocument())
    const [url, init] = fetchMock.mock.calls[1]
    expect(String(url)).toContain('/api/auth/resend-verification')
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({ email: 'a@example.com' })
    // 再送後は同じボタンを押し続けられない
    expect(screen.queryByRole('button', { name: '認証メールを再送' })).toBeNull()
  })

  it('429 のときは上限に達した旨を表示する', async () => {
    const fetchMock = jest
      .fn()
      .mockResolvedValueOnce(jsonResponse(403, { error: 'email_not_verified' }))
      .mockResolvedValueOnce(jsonResponse(429, { error: 'rate limited' }))
    fireEvent.click(await loginAsUnverified(fetchMock))

    await waitFor(() => expect(screen.getByText(/再送の回数が上限/)).toBeInTheDocument())
  })

  it('パスワード誤りなど別のエラーでは再送ボタンを出さない', async () => {
    const fetchMock = jest.fn().mockResolvedValueOnce(jsonResponse(401, { error: 'invalid email or password' }))
    global.fetch = fetchMock as unknown as typeof fetch
    render(<LoginPage onAuthSuccess={jest.fn()} />)
    fireEvent.change(screen.getByLabelText(/メールアドレス/), { target: { value: 'a@example.com' } })
    fireEvent.change(screen.getByLabelText(/パスワード/), { target: { value: 'x' } })
    fireEvent.click(screen.getByRole('button', { name: 'ログイン' }))

    await screen.findByText(/正しくありません/)
    expect(screen.queryByRole('button', { name: '認証メールを再送' })).toBeNull()
  })
})
