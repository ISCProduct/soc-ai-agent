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
// 無限に往復する。以前は失敗時に何もせず、期限切れトークンをそのまま注入して
// Cookieも残していた。ページが401を受けてログイン画面へ飛び、そのログイン画面でも
// Cookieが残っているため再びリフレッシュを試み、抜け出せなかった。
// リフレッシュトークンの有効期間は30日なので、30日以上ぶりにアクセスした利用者は
// 全員これに入る。
//
// 失敗の種類で扱いを分ける（PR #1520のレビュー指摘）。
//   401/403 = 失効が確定 -> Cookieを消す
//   5xx・通信断・タイムアウト = 一時障害 -> Cookieは残す（通信が不安定なだけの
//   利用者を毎回ログアウトさせない）
// どちらの場合も、期限切れの値は後段へ渡さない。X-User-* を止めるだけでは
// app/api/auth/session/route.ts と lib/auth/server.ts がCookieを直接読んでしまうため、
// 後段へ渡すCookieからも外すことを固定する。
describe('middleware: リフレッシュ失敗時のセッション破棄 (#1519)', () => {
  const originalFetch = global.fetch

  // middleware は署名を検証せず exp だけ見てリフレッシュ契機を決める。
  function jwt(expiresInSeconds: number): string {
    const b64 = (o: unknown) =>
      Buffer.from(JSON.stringify(o)).toString('base64url')
    const exp = Math.floor(Date.now() / 1000) + expiresInSeconds
    return `${b64({ alg: 'HS256', typ: 'JWT' })}.${b64({ sub: '1', exp })}.sig`
  }

  const expiredJwt = () => jwt(-3600)

  /** 後段(Route Handler / Server Component)が受け取るCookieヘッダー */
  function forwardedCookie(response: Response): string | null {
    return response.headers.get('x-middleware-request-cookie')
  }

  const DELETED_COOKIE = /Max-Age=0|Expires=Thu, 01 Jan 1970/

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
    expect(setCookie).toMatch(DELETED_COOKIE)
  })

  // /api/auth/session はヘッダーが無いとCookieへフォールバックし、200で古いJWTを
  // 返してしまう。クライアントはそれをストレージへ再保存するので、面接開始や
  // ゲスト登録の直後にBackendが401を返す。
  it('失効した認証Cookieを後段へ渡さない（Cookieフォールバックの抜け道を閉じる）', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, status: 401, json: async () => ({}) })

    const request = new NextRequest('http://localhost:3000/api/auth/session', {
      headers: {
        cookie: `theme=dark; user_id=1; user_token=${expiredJwt()}; refresh_token=stale`,
      },
    })

    const response = await middleware(request)

    const cookie = forwardedCookie(response) ?? ''
    expect(cookie).not.toContain('user_id=')
    expect(cookie).not.toContain('user_token=')
    // 認証に無関係なCookieは落とさない
    expect(cookie).toContain('theme=dark')
  })

  it('一時障害(5xx)ではCookieを残すが、期限切れトークンは後段へ渡さない', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, status: 503, json: async () => ({}) })

    const request = new NextRequest('http://localhost:3000/api/schedule', {
      headers: { cookie: `user_id=1; user_token=${expiredJwt()}; refresh_token=valid` },
    })

    const response = await middleware(request)

    // Backend再起動やDBの一時障害で、まだ30日有効なリフレッシュトークンを捨てさせない
    expect(response.headers.get('set-cookie')).toBeNull()
    // それでも期限切れの値は渡さない（渡すと401が返り続けて再びループする）
    expect(response.headers.get('x-middleware-request-x-user-token')).toBeNull()
    expect(forwardedCookie(response) ?? '').not.toContain('user_token=')
  })

  it('一時障害(通信エラー)でもCookieを残し、期限切れトークンは後段へ渡さない', async () => {
    global.fetch = jest.fn().mockRejectedValue(new TypeError('Failed to fetch'))

    const request = new NextRequest('http://localhost:3000/api/schedule', {
      headers: { cookie: `user_id=1; user_token=${expiredJwt()}; refresh_token=valid` },
    })

    const response = await middleware(request)

    expect(response.headers.get('set-cookie')).toBeNull()
    expect(response.headers.get('x-middleware-request-x-user-token')).toBeNull()
    expect(forwardedCookie(response) ?? '').not.toContain('user_token=')
  })

  // リフレッシュは期限の120秒前から走る。一時障害に当たっただけで、まだ有効な
  // アクセストークンを使えなくするとユーザー操作を止めてしまう。
  it('一時障害でもアクセストークンが未期限なら従来どおり注入する', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({}) })

    const stillValid = jwt(60)
    const request = new NextRequest('http://localhost:3000/api/schedule', {
      headers: { cookie: `user_id=1; user_token=${stillValid}; refresh_token=valid` },
    })

    const response = await middleware(request)

    expect(response.headers.get('x-middleware-request-x-user-token')).toBe(stillValid)
    expect(response.headers.get('set-cookie')).toBeNull()
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

  it('企業ポータル側も401ならCookieを消し、失効値を後段へ渡さない', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, status: 401, json: async () => ({}) })

    const request = new NextRequest('http://localhost:3000/company-portal', {
      headers: {
        cookie: `company_user_id=1; company_user_token=${expiredJwt()}; company_refresh_token=stale`,
      },
    })

    const response = await middleware(request)

    expect(response.headers.get('x-middleware-request-x-company-user-token')).toBeNull()
    const cookie = forwardedCookie(response) ?? ''
    expect(cookie).not.toContain('company_user_id=')
    expect(cookie).not.toContain('company_user_token=')
    const setCookie = response.headers.get('set-cookie') ?? ''
    expect(setCookie).toContain('company_user_token=')
    expect(setCookie).toMatch(DELETED_COOKIE)
  })

  it('企業ポータル側も一時障害ではCookieを残し、失効値は後段へ渡さない', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, status: 502, json: async () => ({}) })

    const request = new NextRequest('http://localhost:3000/company-portal', {
      headers: {
        cookie: `company_user_id=1; company_user_token=${expiredJwt()}; company_refresh_token=valid`,
      },
    })

    const response = await middleware(request)

    expect(response.headers.get('set-cookie')).toBeNull()
    expect(response.headers.get('x-middleware-request-x-company-user-token')).toBeNull()
    expect(forwardedCookie(response) ?? '').not.toContain('company_user_token=')
  })
})
