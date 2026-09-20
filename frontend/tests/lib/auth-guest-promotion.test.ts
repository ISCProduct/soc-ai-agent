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

  // exp 付きの JWT 風トークン。期限内なら register はリフレッシュを挟まない。
  const jwtWithExp = (secondsFromNow: number) => {
    const payload = Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + secondsFromNow })).toString('base64url')
    return `header.${payload}.signature`
  }
  const freshToken = jwtWithExp(60 * 30)
  const expiredToken = jwtWithExp(-60)

  afterEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    jest.restoreAllMocks()
  })

  it('ゲストなら promote_guest と X-User-Token を送る', async () => {
    setStored({ user_id: 7, name: 'Guest', is_guest: true }, freshToken)
    const fetchMock = jest.fn().mockResolvedValue(okResponse)
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.register('s@example.com', 'password123', '山田', '新卒', '', '')

    const [, init] = fetchMock.mock.calls[0]
    const body = JSON.parse(String((init as RequestInit).body))
    const headers = (init as RequestInit).headers as Record<string, string>

    expect(body.promote_guest).toBe(true)
    expect(headers['X-User-Token']).toBe(freshToken)
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

// 本登録はメール往復を挟むので、1時間のアクセストークンは普通に切れる。
// 期限切れのまま送るとサーバが 401 を返し、再読み込みしても localStorage は
// 直らないので登録自体ができなくなる（#1374 のレビュー指摘）。
describe('register の期限切れゲストトークン', () => {
  const okResponse = {
    ok: true,
    json: async () => ({ user_id: 42, email: 's@example.com', name: '山田', is_guest: false }),
  }
  const expired = `header.${Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) - 60 })).toString('base64url')}.sig`
  const refreshed = `header.${Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 1800 })).toString('base64url')}.sig`

  afterEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    jest.restoreAllMocks()
  })

  it('期限切れならリフレッシュして新しいトークンを送る', async () => {
    setStored({ user_id: 7, name: 'Guest', is_guest: true }, expired)
    const fetchMock = jest.fn().mockImplementation(async (url: string) => {
      if (String(url).includes('/api/auth/session')) {
        return { ok: true, json: async () => ({ user_token: refreshed }) } as unknown as Response
      }
      return okResponse as unknown as Response
    })
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.register('s@example.com', 'password123', '山田', '新卒', '', '')

    const registerCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/register'))!
    const headers = (registerCall[1] as RequestInit).headers as Record<string, string>
    const body = JSON.parse(String((registerCall[1] as RequestInit).body))

    expect(body.promote_guest).toBe(true)
    expect(headers['X-User-Token']).toBe(refreshed)
  })

  // リフレッシュも切れていればサーバ側でもゲストを特定できない。
  // 引き継ぎは諦めるが、登録自体は通す（止めても学生にできることが無い）。
  it('リフレッシュも失敗したら引き継がずに登録する', async () => {
    setStored({ user_id: 7, name: 'Guest', is_guest: true }, expired)
    const fetchMock = jest.fn().mockImplementation(async (url: string) => {
      if (String(url).includes('/api/auth/session')) {
        return { ok: false, json: async () => ({}) } as unknown as Response
      }
      return okResponse as unknown as Response
    })
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.register('s@example.com', 'password123', '山田', '新卒', '', '')

    const registerCall = fetchMock.mock.calls.find(([url]) => String(url).includes('/api/auth/register'))!
    const headers = (registerCall[1] as RequestInit).headers as Record<string, string>
    const body = JSON.parse(String((registerCall[1] as RequestInit).body))

    expect(body.promote_guest).toBe(false)
    expect(headers['X-User-Token']).toBeUndefined()
  })
})
