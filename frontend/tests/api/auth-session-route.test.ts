import { NextRequest } from 'next/server'
import { DELETE, GET, POST } from '@/app/api/auth/session/route'

describe('GET /api/auth/session の管理者トークン', () => {
  it('このリクエストで取り直した値を、古いCookieより優先する', async () => {
    const request = new NextRequest('http://localhost:3000/api/auth/session', {
      headers: {
        'X-User-ID': '1',
        'X-User-Token': 'user-jwt',
        'X-Refreshed-Admin-Token': 'fresh-admin',
        cookie: 'admin_token=stale-admin',
      },
    })
    const response = await GET(request)
    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toEqual({
      user_id: 1,
      user_token: 'user-jwt',
      token: 'fresh-admin',
    })
  })
})

describe('POST /api/auth/session の管理者トークン', () => {
  it('ログイン時の管理者トークンをCookieへ残す', async () => {
    const request = new NextRequest('http://localhost:3000/api/auth/session', {
      method: 'POST',
      body: JSON.stringify({
        userId: 1,
        userToken: 'user-jwt',
        refreshToken: 'refresh',
        adminToken: 'issued-admin',
      }),
    })
    const response = await POST(request)
    expect(response.status).toBe(200)
    expect(response.headers.get('set-cookie') ?? '').toContain('admin_token=issued-admin')
  })
})

describe('管理者トークンCookieの掃除', () => {
  it('ログイン応答に管理者トークンが無ければ、前の利用者の admin_token を消す', async () => {
    const request = new NextRequest('http://localhost:3000/api/auth/session', {
      method: 'POST',
      headers: { cookie: 'admin_token=previous-admin' },
      body: JSON.stringify({ userId: 2, userToken: 'student-jwt', refreshToken: 'refresh' }),
    })
    const response = await POST(request)
    expect(response.status).toBe(200)
    expect(response.headers.get('set-cookie') ?? '').toMatch(/admin_token=;/)
  })

  it('ログアウトで admin_token も消す', async () => {
    const request = new NextRequest('http://localhost:3000/api/auth/session', {
      method: 'DELETE',
      headers: { cookie: 'admin_token=issued-admin' },
    })
    const response = await DELETE(request)
    expect(response.status).toBe(200)
    expect(response.headers.get('set-cookie') ?? '').toMatch(/admin_token=;/)
  })
})
