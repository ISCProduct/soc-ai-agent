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
})
