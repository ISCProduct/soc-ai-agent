/**
 * @jest-environment jsdom
 */
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
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

describe('LoginPage のタブと中身の対応', () => {
  function panelOf(tab: HTMLElement): HTMLElement {
    const panelId = tab.getAttribute('aria-controls')
    expect(panelId).toBeTruthy()
    const panel = document.getElementById(panelId as string)
    expect(panel).not.toBeNull()
    return panel as HTMLElement
  }

  it('各タブの aria-controls が tabpanel を指し、tabpanel の aria-labelledby がそのタブを指す', () => {
    render(<LoginPage onAuthSuccess={jest.fn()} />)

    const tabs = screen.getAllByRole('tab')
    expect(tabs.map((t) => t.textContent)).toEqual(['ログイン', '新規登録'])
    for (const tab of tabs) {
      const panel = panelOf(tab)
      expect(panel).toHaveAttribute('role', 'tabpanel')
      expect(panel).toHaveAttribute('aria-labelledby', tab.id)
    }
    expect(panelOf(tabs[0]).id).not.toBe(panelOf(tabs[1]).id)
  })

  it('選ばれているタブの中身だけが表示され、タブを切り替えると対応する中身に替わる', () => {
    render(<LoginPage onAuthSuccess={jest.fn()} />)
    const [loginTab, registerTab] = screen.getAllByRole('tab')

    expect(loginTab).toHaveAttribute('aria-selected', 'true')
    expect(panelOf(loginTab)).toBeVisible()
    expect(within(panelOf(loginTab)).getByRole('button', { name: 'ログイン' })).toBeInTheDocument()
    expect(panelOf(registerTab)).not.toBeVisible()

    fireEvent.click(registerTab)
    expect(registerTab).toHaveAttribute('aria-selected', 'true')
    expect(panelOf(registerTab)).toBeVisible()
    expect(within(panelOf(registerTab)).getByRole('button', { name: '確認メールを送る' })).toBeInTheDocument()
    expect(panelOf(loginTab)).not.toBeVisible()
    expect(screen.queryByRole('button', { name: 'ログイン' })).toBeNull()
  })

  it('initialTab=1 では新規登録のタブと中身が選ばれている', () => {
    render(<LoginPage onAuthSuccess={jest.fn()} initialTab={1} />)
    const [loginTab, registerTab] = screen.getAllByRole('tab')

    expect(registerTab).toHaveAttribute('aria-selected', 'true')
    expect(panelOf(registerTab)).toBeVisible()
    expect(panelOf(loginTab)).not.toBeVisible()
  })

  it('左右の矢印キーで隣のタブへ移る', async () => {
    render(<LoginPage onAuthSuccess={jest.fn()} />)
    const [loginTab, registerTab] = screen.getAllByRole('tab')

    await act(async () => loginTab.focus())
    await act(async () => {
      fireEvent.keyDown(loginTab, { key: 'ArrowRight' })
    })
    expect(registerTab).toHaveFocus()

    await act(async () => {
      fireEvent.keyDown(registerTab, { key: 'ArrowLeft' })
    })
    expect(loginTab).toHaveFocus()
  })
})
