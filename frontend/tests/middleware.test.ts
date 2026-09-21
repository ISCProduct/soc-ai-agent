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
