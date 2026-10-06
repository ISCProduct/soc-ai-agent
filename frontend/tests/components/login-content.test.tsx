/**
 * @jest-environment jsdom
 */
import { render, screen, waitFor } from '@testing-library/react'
import { LoginContent } from '@/components/LoginContent'
import { authService } from '@/lib/auth'

const replace = jest.fn()
const push = jest.fn()
let tabParam: string | null = null

jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace, push }),
  useSearchParams: () => ({ get: (key: string) => (key === 'tab' ? tabParam : null) }),
}))

// ログインフォーム本体はこのテストの対象外。遷移の判断だけを見る
jest.mock('@/components/LoginPage', () => ({
  LoginPage: () => <div data-testid="login-form" />,
}))

// jsdom には fetch/Response が無いため、必要な部分だけを持つ応答を組み立てる
function jsonResponse(status: number, body: unknown) {
  return { ok: status >= 200 && status < 300, status, json: async () => body }
}

function storeUser(user: Record<string, unknown>) {
  window.localStorage.setItem('user', JSON.stringify(user))
  window.localStorage.setItem('user_token', 'stored-token')
}

describe('LoginContent のセッション判定', () => {
  beforeEach(() => {
    replace.mockClear()
    push.mockClear()
    tabParam = null
    window.localStorage.clear()
    window.sessionStorage.clear()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('Cookieのセッションが失効していると、/ へ送らずログイン画面に留まる（#1519 の往復ループ）', async () => {
    // 30日ぶりのアクセスで middleware がCookieを消した状態。
    // localStorage にはユーザーが残っている。
    storeUser({ user_id: 1, email: 'a@example.com', name: 'A', is_guest: false })

    const fetchMock = jest.fn(async () => jsonResponse(401, { error: 'unauthorized' }))
    global.fetch = fetchMock as unknown as typeof fetch

    render(<LoginContent />)

    await waitFor(() => expect(fetchMock).toHaveBeenCalled())
    // / へ送るとサーバー側が再び /login へ戻し、ローディングとログイン画面を往復する
    await waitFor(() => expect(replace).not.toHaveBeenCalled())
    expect(screen.getByTestId('login-form')).toBeInTheDocument()
  })

  it('401でもログアウトAPIは叩かない（Backend一時障害でも401になるため #1519）', async () => {
    storeUser({ user_id: 1, email: 'a@example.com', name: 'A', is_guest: false })

    const calls: Array<{ url: string; method?: string }> = []
    global.fetch = jest.fn(async (input: unknown, init?: RequestInit) => {
      calls.push({ url: String(input), method: init?.method })
      return jsonResponse(401, { error: 'unauthorized' })
    }) as unknown as typeof fetch

    render(<LoginContent />)

    await waitFor(() => expect(calls.length).toBeGreaterThan(0))
    // DELETE は Backend の /api/auth/logout を呼び、まだ30日有効な
    // リフレッシュトークンを失効させてしまう
    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE')).toBe(false))
    expect(authService.getStoredUser()).not.toBeNull()
  })

  it('Cookieのセッションが生きていれば / へ送る', async () => {
    storeUser({ user_id: 1, email: 'a@example.com', name: 'A', is_guest: false })

    global.fetch = jest.fn(async () =>
      jsonResponse(200, { user_id: 1, user_token: 'fresh' }),
    ) as unknown as typeof fetch

    render(<LoginContent />)

    await waitFor(() => expect(replace).toHaveBeenCalledWith('/'))
  })

  it('通信できないときはログイン画面に留まる（判断できないのでループさせない）', async () => {
    storeUser({ user_id: 1, email: 'a@example.com', name: 'A', is_guest: false })

    global.fetch = jest.fn(async () => {
      throw new Error('network down')
    }) as unknown as typeof fetch

    render(<LoginContent />)

    await waitFor(() => expect(screen.getByTestId('login-form')).toBeInTheDocument())
    expect(replace).not.toHaveBeenCalled()
  })

  it('保存されたユーザーが無ければセッションを問い合わせない', async () => {
    const fetchMock = jest.fn()
    global.fetch = fetchMock as unknown as typeof fetch

    render(<LoginContent />)

    await waitFor(() => expect(screen.getByTestId('login-form')).toBeInTheDocument())
    expect(fetchMock).not.toHaveBeenCalled()
    expect(replace).not.toHaveBeenCalled()
  })

  it('ゲストが登録タブから来たらセッションを捨てて登録画面を出す（既存挙動）', async () => {
    tabParam = 'register'
    storeUser({ user_id: 2, email: 'g@example.com', name: 'G', is_guest: true })

    const fetchMock = jest.fn(async () => jsonResponse(200, {}))
    global.fetch = fetchMock as unknown as typeof fetch

    render(<LoginContent />)

    await waitFor(() => expect(authService.getStoredUser()).toBeNull())
    expect(replace).not.toHaveBeenCalled()
  })
})
