/**
 * @jest-environment jsdom
 */
import { authService } from '@/lib/auth'

function setStored(user: unknown, token: string | null) {
  localStorage.clear()
  sessionStorage.clear()
  if (user !== null) localStorage.setItem('user', JSON.stringify(user))
  if (token !== null) localStorage.setItem('user_token', token)
}

describe('isGuestSession', () => {
  afterEach(() => {
    localStorage.clear()
    sessionStorage.clear()
  })

  it('ゲストでトークンがあれば true', () => {
    setStored({ user_id: 1, name: 'Guest', is_guest: true }, 'tok')
    expect(authService.isGuestSession()).toBe(true)
  })

  // 本登録済みの利用者が別アカウントを作る経路を壊さない。
  it('本登録済みなら false', () => {
    setStored({ user_id: 1, name: '山田', is_guest: false }, 'tok')
    expect(authService.isGuestSession()).toBe(false)
  })

  // トークンが無いとサーバ側で対象を特定できず、登録が401で失敗する。
  // 引き継ぎを申告しないことで、通常登録として成立させる。
  it('トークンが無ければ false', () => {
    setStored({ user_id: 1, name: 'Guest', is_guest: true }, null)
    expect(authService.isGuestSession()).toBe(false)
  })

  it('保存が無ければ false', () => {
    setStored(null, null)
    expect(authService.isGuestSession()).toBe(false)
  })
})

describe('register のゲスト引き継ぎ', () => {
  const okResponse = {
    ok: true,
    json: async () => ({ user_id: 42, email: 's@example.com', name: '山田', is_guest: false }),
  }

  afterEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    jest.restoreAllMocks()
  })

  it('ゲストなら promote_guest と X-User-Token を送る', async () => {
    setStored({ user_id: 7, name: 'Guest', is_guest: true }, 'guest-token')
    const fetchMock = jest.fn().mockResolvedValue(okResponse)
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.register('s@example.com', 'password123', '山田', '新卒', '', '')

    const [, init] = fetchMock.mock.calls[0]
    const body = JSON.parse(String((init as RequestInit).body))
    const headers = (init as RequestInit).headers as Record<string, string>

    expect(body.promote_guest).toBe(true)
    expect(headers['X-User-Token']).toBe('guest-token')
    // 対象はトークンから決まる。ボディにIDを入れない。
    expect(body.user_id).toBeUndefined()
  })

  it('ゲストでなければ promote_guest を立てずトークンも送らない', async () => {
    setStored({ user_id: 7, name: '山田', is_guest: false }, 'some-token')
    const fetchMock = jest.fn().mockResolvedValue(okResponse)
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.register('s@example.com', 'password123', '山田', '新卒', '', '')

    const [, init] = fetchMock.mock.calls[0]
    const body = JSON.parse(String((init as RequestInit).body))
    const headers = (init as RequestInit).headers as Record<string, string>

    expect(body.promote_guest).toBe(false)
    expect(headers['X-User-Token']).toBeUndefined()
  })
})
