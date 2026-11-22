import { NextRequest } from 'next/server'
import { DELETE, GET, POST } from '@/app/api/auth/session/route'
import { POST as COMPANY_POST } from '@/app/api/company-auth/session/route'

// 自サイトの fetch が付けるヘッダ
const SAME_SITE = { 'content-type': 'application/json', origin: 'http://localhost:3000' }

describe('GET /api/auth/session の管理者トークン', () => {
  it('管理者トークンの値は返さず、有無だけを返す（SOCAIAGENT-683）', async () => {
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
      has_admin_token: true,
    })
  })
})

describe('POST /api/auth/session の管理者トークン', () => {
  it('ログイン時の管理者トークンをCookieへ残す', async () => {
    const request = new NextRequest('http://localhost:3000/api/auth/session', {
      method: 'POST',
      headers: SAME_SITE,
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
      headers: { ...SAME_SITE, cookie: 'admin_token=previous-admin' },
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

describe('セッションCookie発行 POST の呼び出し元検証 (SOCAIAGENT-684)', () => {
  const routes = [
    { name: 'auth', post: POST, url: 'http://localhost:3000/api/auth/session', body: { userId: 1, userToken: 'jwt' } },
    {
      name: 'company-auth',
      post: COMPANY_POST,
      url: 'http://localhost:3000/api/company-auth/session',
      body: { companyUserId: 1, companyUserToken: 'jwt' },
    },
  ]
  const cases: { name: string; headers: Record<string, string>; raw?: string; status: number }[] = [
    { name: '自サイトの JSON は通す', headers: SAME_SITE, status: 200 },
    { name: 'charset 付きの JSON も通す', headers: { ...SAME_SITE, 'content-type': 'application/json; charset=utf-8' }, status: 200 },
    { name: 'text/plain のフォーム送信は 415', headers: { ...SAME_SITE, 'content-type': 'text/plain' }, status: 415 },
    { name: 'Content-Type なしは 415', headers: { origin: 'http://localhost:3000' }, status: 415 },
    { name: '他サイトの Origin は 403', headers: { ...SAME_SITE, origin: 'https://evil.example' }, status: 403 },
    { name: 'ポート違いの Origin は 403', headers: { ...SAME_SITE, origin: 'http://localhost:4000' }, status: 403 },
    { name: 'Origin なしは 403', headers: { 'content-type': 'application/json' }, status: 403 },
    { name: '不正な Origin は 403', headers: { ...SAME_SITE, origin: 'null' }, status: 403 },
    { name: 'サブドメインの Origin は 403', headers: { ...SAME_SITE, origin: 'http://sub.localhost:3000' }, status: 403 },
    {
      name: 'X-Forwarded-Host を合わせても Host と違えば 403',
      headers: { ...SAME_SITE, origin: 'https://evil.example', 'x-forwarded-host': 'evil.example' },
      status: 403,
    },
    { name: '大文字の Origin は正規化して通す', headers: { ...SAME_SITE, origin: 'http://LOCALHOST:3000' }, status: 200 },
    { name: '壊れた JSON は 400', headers: SAME_SITE, raw: '{', status: 400 },
    { name: '配列の JSON は 400', headers: SAME_SITE, raw: '[]', status: 400 },
  ]

  for (const route of routes) {
    for (const c of cases) {
      it(`${route.name}: ${c.name}`, async () => {
        const warn = jest.spyOn(console, 'warn').mockImplementation(() => {})
        const request = new NextRequest(route.url, {
          method: 'POST',
          headers: c.headers,
          body: c.raw ?? JSON.stringify(route.body),
        })
        const response = await route.post(request)
        expect(response.status).toBe(c.status)
        const setCookie = response.headers.get('set-cookie') ?? ''
        if (c.status === 200) {
          expect(setCookie).toContain('jwt')
        } else {
          expect(setCookie).toBe('')
          // 拒否ログにトークンの値を出さない
          expect(warn.mock.calls.flat().join(' ')).not.toContain('jwt')
        }
        warn.mockRestore()
      })
    }
  }
})
