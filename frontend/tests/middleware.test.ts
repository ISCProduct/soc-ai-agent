import { NextRequest } from 'next/server'
import { middleware } from '@/middleware'

// #987: middleware.tsはcookie不在時でもクライアント送信のX-User-ID/X-User-Token/
// X-Tenant-Slugヘッダーを必ず除去すること(以前はそのまま後段のRoute Handlerへ
// 通過してしまっていた)。

describe('middleware', () => {
  it('cookie不在時、クライアント送信のなりすましヘッダーを除去する', async () => {
    const request = new NextRequest('http://localhost:3000/api/schedule', {
      headers: {
        'X-User-ID': '999',
        'X-User-Token': 'forged-token',
        'X-Tenant-Slug': 'forged-tenant',
      },
    })

    const response = await middleware(request)

    expect(response.headers.get('x-middleware-request-x-user-id')).toBeNull()
    expect(response.headers.get('x-middleware-request-x-user-token')).toBeNull()
    expect(response.headers.get('x-middleware-request-x-tenant-slug')).toBeNull()
  })

  it('有効なセッションcookieがある場合はcookie由来の値で上書きする', async () => {
    const request = new NextRequest('http://localhost:3000/api/schedule', {
      headers: {
        'X-User-ID': '999',
        'X-User-Token': 'forged-token',
        cookie: 'user_id=1; user_token=real-token',
      },
    })

    const response = await middleware(request)

    expect(response.headers.get('x-middleware-request-x-user-id')).toBe('1')
    expect(response.headers.get('x-middleware-request-x-user-token')).toBe('real-token')
  })

  // 企業相関図の旧URL。next.config の redirects() は source の照合が大文字小文字を
  // 区別せず、新URL自身もマッチして無限リダイレクトになる。middleware で厳密比較
  // している根拠をここで固定する。
  it('旧URL /Correlation-diagram を小文字へ308でリダイレクトする', async () => {
    const request = new NextRequest('http://localhost:3000/Correlation-diagram?company_id=42')
    const response = await middleware(request)
    expect(response.status).toBe(308)
    const location = new URL(response.headers.get('location') as string)
    expect(location.pathname).toBe('/correlation-diagram')
    // company_id を落とすと遷移先で対象企業が選ばれない
    expect(location.searchParams.get('company_id')).toBe('42')
  })

  it('新URL /correlation-diagram はリダイレクトしない（無限ループ防止）', async () => {
    const request = new NextRequest('http://localhost:3000/correlation-diagram')
    const response = await middleware(request)
    expect(response.status).not.toBe(308)
    expect(response.headers.get('location')).toBeNull()
  })
})

// #1519: リフレッシュ失敗時にCookieを残すと、ログイン画面とローディングを
// 無限に往復する。期限切れトークンをヘッダーへ注入しないこと、Cookieを消すことを固定する。
//
// 以前は失敗時に何もせず、期限切れトークンをそのまま注入してCookieも残していた。
// ページが401を受けてログイン画面へ飛び、そのログイン画面でもCookieが残っている
// ため再びリフレッシュを試み、抜け出せなかった。リフレッシュトークンの有効期間は
// 30日なので、30日以上ぶりにアクセスした利用者は全員これに入る。
describe('middleware: リフレッシュ失敗時のセッション破棄 (#1519)', () => {
  const originalFetch = global.fetch

  // exp が過去のJWT。middleware は署名を検証せず exp だけ見てリフレッシュ契機を決める。
  function expiredJwt(): string {
    const b64 = (o: unknown) =>
      Buffer.from(JSON.stringify(o)).toString('base64url')
    return `${b64({ alg: 'HS256', typ: 'JWT' })}.${b64({ sub: '1', exp: Math.floor(Date.now() / 1000) - 3600 })}.sig`
  }

  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('リフレッシュが401なら期限切れトークンを注入せずCookieを消す', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, status: 401, json: async () => ({}) })

    const request = new NextRequest('http://localhost:3000/api/schedule', {
      headers: { cookie: `user_id=1; user_token=${expiredJwt()}; refresh_token=stale` },
    })

    const response = await middleware(request)

    // 期限切れトークンを後段へ流すと、Backendが401を返し続けてループになる
    expect(response.headers.get('x-middleware-request-x-user-id')).toBeNull()
    expect(response.headers.get('x-middleware-request-x-user-token')).toBeNull()

    // Cookieを消さないと次のリクエストでも同じリフレッシュを試みる
    const setCookie = response.headers.get('set-cookie') ?? ''
    expect(setCookie).toContain('user_id=')
    expect(setCookie).toContain('user_token=')
    expect(setCookie).toContain('refresh_token=')
    expect(setCookie).toMatch(/Max-Age=0|Expires=Thu, 01 Jan 1970/)
  })

  it('リフレッシュが通信エラーでもCookieを消す', async () => {
    global.fetch = jest.fn().mockRejectedValue(new TypeError('Failed to fetch'))

    const request = new NextRequest('http://localhost:3000/api/schedule', {
      headers: { cookie: `user_id=1; user_token=${expiredJwt()}; refresh_token=stale` },
    })

    const response = await middleware(request)

    expect(response.headers.get('x-middleware-request-x-user-token')).toBeNull()
    expect(response.headers.get('set-cookie') ?? '').toMatch(/Max-Age=0|Expires=Thu, 01 Jan 1970/)
  })

  it('リフレッシュが成功すれば新しいトークンを注入しCookieも更新する（後方互換）', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ user_id: 1, user_token: 'fresh-token', refresh_token: 'fresh-refresh' }),
    })

    const request = new NextRequest('http://localhost:3000/api/schedule', {
      headers: { cookie: `user_id=1; user_token=${expiredJwt()}; refresh_token=ok` },
    })

    const response = await middleware(request)

    expect(response.headers.get('x-middleware-request-x-user-token')).toBe('fresh-token')
    expect(response.headers.get('set-cookie') ?? '').toContain('fresh-refresh')
  })

  it('企業ポータル側も同様にCookieを消す', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, status: 401, json: async () => ({}) })

    const request = new NextRequest('http://localhost:3000/company-portal', {
      headers: {
        cookie: `company_user_id=1; company_user_token=${expiredJwt()}; company_refresh_token=stale`,
      },
    })

    const response = await middleware(request)

    expect(response.headers.get('x-middleware-request-x-company-user-token')).toBeNull()
    const setCookie = response.headers.get('set-cookie') ?? ''
    expect(setCookie).toContain('company_user_token=')
    expect(setCookie).toMatch(/Max-Age=0|Expires=Thu, 01 Jan 1970/)
  })
})
