import { createHash } from 'node:crypto'
import { NextRequest } from 'next/server'
import { GET as START } from '@/app/api/auth/oauth/start/route'
import { POST as EXCHANGE } from '@/app/api/auth/oauth/exchange/route'

const SAME_SITE = { 'content-type': 'application/json', origin: 'http://localhost:3000' }

function cookieValue(setCookie: string, name: string): string | undefined {
  return setCookie.match(new RegExp(`${name}=([^;]*)`))?.[1]
}

describe('GET /api/auth/oauth/start (SOCAIAGENT-683)', () => {
  it('verifier を httpOnly Cookie に入れ、そのハッシュを bind として Backend の OAuth 開始へ送る', async () => {
    const response = await START(new NextRequest('http://localhost:3000/api/auth/oauth/start?provider=google&tenant=acme'))
    expect(response.status).toBe(307)

    const setCookie = response.headers.get('set-cookie') ?? ''
    expect(setCookie).toMatch(/oauth_verifier=[^;]+;.*HttpOnly/i)
    const verifier = cookieValue(setCookie, 'oauth_verifier') ?? ''
    expect(verifier.length).toBeGreaterThanOrEqual(43)

    const location = new URL(response.headers.get('location') ?? '')
    expect(location.pathname).toBe('/api/auth/google')
    expect(location.searchParams.get('tenant')).toBe('acme')
    // Backend の authsvc.OAuthBindHash と同じ形式
    expect(location.searchParams.get('bind')).toBe(createHash('sha256').update(verifier).digest('base64url'))
    // verifier そのものは URL に載せない
    expect(location.toString()).not.toContain(verifier)
  })

  it('開始のたびに別の verifier を作る', async () => {
    const a = await START(new NextRequest('http://localhost:3000/api/auth/oauth/start?provider=github'))
    const b = await START(new NextRequest('http://localhost:3000/api/auth/oauth/start?provider=github'))
    expect(cookieValue(a.headers.get('set-cookie') ?? '', 'oauth_verifier')).not.toBe(
      cookieValue(b.headers.get('set-cookie') ?? '', 'oauth_verifier')
    )
  })

  it('知らないプロバイダは 400', async () => {
    const response = await START(new NextRequest('http://localhost:3000/api/auth/oauth/start?provider=evil'))
    expect(response.status).toBe(400)
    expect(response.headers.get('set-cookie')).toBeNull()
  })
})

describe('POST /api/auth/oauth/exchange (SOCAIAGENT-683)', () => {
  const backendAuth = { user_id: 7, email: 'a@example.com', name: '学生', user_token: 'user-jwt', refresh_token: 'refresh' }
  let fetchMock: jest.Mock
  let warn: jest.SpyInstance

  beforeEach(() => {
    fetchMock = jest.fn()
    global.fetch = fetchMock
    warn = jest.spyOn(console, 'warn').mockImplementation(() => {})
  })
  afterEach(() => warn.mockRestore())

  function exchange(headers: Record<string, string>, body: unknown = { code: 'short-code' }) {
    return EXCHANGE(
      new NextRequest('http://localhost:3000/api/auth/oauth/exchange', {
        method: 'POST',
        headers,
        body: JSON.stringify(body),
      })
    )
  }

  it('Cookie の verifier とコードを Backend に渡し、セッション Cookie を張って verifier を消す', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => backendAuth })
    const response = await exchange({ ...SAME_SITE, cookie: 'oauth_verifier=my-verifier; admin_token=previous-admin' })

    expect(response.status).toBe(200)
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ code: 'short-code', verifier: 'my-verifier' })
    // トークンは Cookie にだけ置き、画面へは返さない（SOCAIAGENT-683 PR3b）
    const body = await response.json()
    expect(body).not.toHaveProperty('user_token')
    expect(body).not.toHaveProperty('refresh_token')
    expect(body.user_id).toBe(7)
    const setCookie = response.headers.get('set-cookie') ?? ''
    expect(setCookie).toContain('user_token=user-jwt')
    expect(setCookie).toContain('refresh_token=refresh')
    expect(setCookie).toMatch(/oauth_verifier=;/)
    // 管理者でなければ前の利用者の admin_token を残さない
    expect(setCookie).toMatch(/admin_token=;/)
  })

  it('管理者なら admin_token も張る', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ ...backendAuth, token: 'admin-jwt' }) })
    const response = await exchange({ ...SAME_SITE, cookie: 'oauth_verifier=v' })
    expect(response.headers.get('set-cookie') ?? '').toContain('admin_token=admin-jwt')
  })

  it('verifier の Cookie が無い（別のブラウザで始まったログイン）なら Backend に問い合わせず 400', async () => {
    const response = await exchange(SAME_SITE)
    expect(response.status).toBe(400)
    expect(fetchMock).not.toHaveBeenCalled()
    expect(response.headers.get('set-cookie') ?? '').not.toContain('user_token=')
  })

  it('Backend が拒否したらセッション Cookie を張らない', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 400, json: async () => ({ error: 'invalid' }) })
    const response = await exchange({ ...SAME_SITE, cookie: 'oauth_verifier=v' })
    expect(response.status).toBe(400)
    const setCookie = response.headers.get('set-cookie') ?? ''
    expect(setCookie).not.toContain('user_token=')
    expect(setCookie).toMatch(/oauth_verifier=;/)
  })

  it('Backend に届かなければ 502', async () => {
    fetchMock.mockRejectedValue(new Error('down'))
    const response = await exchange({ ...SAME_SITE, cookie: 'oauth_verifier=v' })
    expect(response.status).toBe(502)
  })

  it('他サイトからの呼び出しは 403（#2499 の検証を通す）', async () => {
    const response = await exchange({ ...SAME_SITE, origin: 'https://evil.example', cookie: 'oauth_verifier=v' })
    expect(response.status).toBe(403)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('code が無ければ 400', async () => {
    const response = await exchange({ ...SAME_SITE, cookie: 'oauth_verifier=v' }, {})
    expect(response.status).toBe(400)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('拒否ログにコードや verifier の値を出さない', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 400, json: async () => ({}) })
    await exchange({ ...SAME_SITE, cookie: 'oauth_verifier=secret-verifier' }, { code: 'secret-code' })
    const logged = warn.mock.calls.flat().join(' ')
    expect(logged).not.toContain('secret-verifier')
    expect(logged).not.toContain('secret-code')
  })
})
