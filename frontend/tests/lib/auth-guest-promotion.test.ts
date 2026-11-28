/**
 * @jest-environment jsdom
 */
import { authService } from '@/lib/auth'

function setStoredUser(user: unknown) {
  localStorage.clear()
  sessionStorage.clear()
  if (user !== null) localStorage.setItem('user', JSON.stringify(user))
}

// トークンは httpOnly Cookie にだけ置く（SOCAIAGENT-683）。ゲストかどうかは保存したユーザーで、
// セッションが生きているかは register が GET /api/auth/session で確かめる。
describe('isGuestSession', () => {
  afterEach(() => {
    localStorage.clear()
    sessionStorage.clear()
  })

  it('ゲストなら true', () => {
    setStoredUser({ user_id: 1, name: 'Guest', is_guest: true })
    expect(authService.isGuestSession()).toBe(true)
  })

  // 本登録済みの利用者が別アカウントを作る経路を壊さない。
  it('本登録済みなら false', () => {
    setStoredUser({ user_id: 1, name: '山田', is_guest: false })
    expect(authService.isGuestSession()).toBe(false)
  })

  it('保存が無ければ false', () => {
    setStoredUser(null)
    expect(authService.isGuestSession()).toBe(false)
  })
})

describe('register のゲスト引き継ぎ', () => {
  const okResponse = {
    ok: true,
    json: async () => ({ user_id: 42, email: 's@example.com', name: '山田', is_guest: false }),
  }

  function mockFetch(sessionOk: boolean) {
    const fetchMock = jest.fn().mockImplementation(async (url: string) => {
      if (String(url).includes('/api/auth/session')) {
        return { ok: sessionOk, json: async () => ({ user_id: 7, has_admin_token: false }) } as unknown as Response
      }
      return okResponse as unknown as Response
    })
    global.fetch = fetchMock as unknown as typeof fetch
    return fetchMock
  }

  function registerCall(fetchMock: jest.Mock) {
    const call = fetchMock.mock.calls.find(([url]) => String(url).includes('/auth/register'))!
    return {
      url: String(call[0]),
      body: JSON.parse(String((call[1] as RequestInit).body)),
      headers: (call[1] as RequestInit).headers as Record<string, string>,
    }
  }

  afterEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    jest.restoreAllMocks()
  })

  it('ゲストでセッションが生きていれば promote_guest を立て、中継ルートに送る（トークンは画面から送らない）', async () => {
    setStoredUser({ user_id: 7, name: 'Guest', is_guest: true })
    const fetchMock = mockFetch(true)

    await authService.register('s@example.com', 'password123', '山田', '新卒', '', '')

    const { url, body, headers } = registerCall(fetchMock)
    expect(url).toBe('/api/bff/auth/register')
    expect(body.promote_guest).toBe(true)
    expect(headers['X-User-Token']).toBeUndefined()
    // 対象は Cookie のトークンから決まる。ボディにIDを入れない。
    expect(body.user_id).toBeUndefined()
  })

  it('ゲストでなければ promote_guest を立てず、セッションも確かめない', async () => {
    setStoredUser({ user_id: 7, name: '山田', is_guest: false })
    const fetchMock = mockFetch(true)

    await authService.register('s@example.com', 'password123', '山田', '新卒', '', '')

    expect(registerCall(fetchMock).body.promote_guest).toBe(false)
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes('/api/auth/session'))).toBe(false)
  })

  // リフレッシュも切れていればサーバ側でもゲストを特定できない。
  // 引き継ぎは諦めるが、登録自体は通す（止めても学生にできることが無い）。
  it('ゲストのセッションが切れていたら引き継がずに登録する', async () => {
    setStoredUser({ user_id: 7, name: 'Guest', is_guest: true })
    const fetchMock = mockFetch(false)

    await authService.register('s@example.com', 'password123', '山田', '新卒', '', '')

    expect(registerCall(fetchMock).body.promote_guest).toBe(false)
  })
})
