import { NextRequest } from 'next/server'
import { POST as companyLogin } from '@/app/api/company-auth/login/route'
import { POST as companyRegister } from '@/app/api/company-auth/register/route'
import { POST as companyAcceptInvite } from '@/app/api/company-auth/accept-invite/route'
import { POST as companyResetPassword } from '@/app/api/company-auth/reset-password/route'
import { POST as profile } from '@/app/api/auth/profile/route'
import { POST as bff } from '@/app/api/bff/[...path]/route'

/**
 * セッション Cookie を発行するルートは、他サイトからのフォーム送信（text/plain・別 Origin）を
 * Backend へ渡す前に落とす。落とさないと、攻撃者のアカウントで被害者のセッションを置き換えられる。
 */
type Handler = (request: NextRequest) => Promise<Response>

const bffAt = (path: string): Handler => (request) => bff(request, { params: Promise.resolve({ path: path.split('/') }) })

const routes: { name: string; url: string; handler: Handler }[] = [
  { name: 'company-auth/login', url: '/api/company-auth/login', handler: companyLogin },
  { name: 'company-auth/register', url: '/api/company-auth/register', handler: companyRegister },
  { name: 'company-auth/accept-invite', url: '/api/company-auth/accept-invite', handler: companyAcceptInvite },
  { name: 'company-auth/reset-password', url: '/api/company-auth/reset-password', handler: companyResetPassword },
  { name: 'auth/profile', url: '/api/auth/profile', handler: profile },
  { name: 'bff auth/login', url: '/api/bff/auth/login', handler: bffAt('auth/login') },
  { name: 'bff auth/register', url: '/api/bff/auth/register', handler: bffAt('auth/register') },
  { name: 'bff auth/guest', url: '/api/bff/auth/guest', handler: bffAt('auth/guest') },
]

function post(url: string, headers: Record<string, string>): NextRequest {
  return new NextRequest(`http://localhost:3000${url}`, {
    method: 'POST',
    headers: { host: 'localhost:3000', ...headers },
    body: JSON.stringify({ email: 'hr@example.com', password: 'pw' }),
  })
}

describe('セッションを発行する POST の他サイト送信拒否', () => {
  const fetchMock = jest.fn()
  beforeEach(() => {
    fetchMock.mockReset()
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ message: 'ok' }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    global.fetch = fetchMock as unknown as typeof fetch
    jest.spyOn(console, 'warn').mockImplementation(() => {})
  })
  afterEach(() => jest.restoreAllMocks())

  it.each(routes)('$name: text/plain のフォーム送信は 415 で Backend へ渡さない', async ({ url, handler }) => {
    const res = await handler(post(url, { 'content-type': 'text/plain', origin: 'http://localhost:3000' }))
    expect(res.status).toBe(415)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it.each(routes)('$name: 別 Origin からの送信は 403 で Backend へ渡さない', async ({ url, handler }) => {
    const res = await handler(post(url, { 'content-type': 'application/json', origin: 'https://evil.example' }))
    expect(res.status).toBe(403)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it.each(routes)('$name: Origin が無い送信も 403', async ({ url, handler }) => {
    const res = await handler(post(url, { 'content-type': 'application/json' }))
    expect(res.status).toBe(403)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it.each(routes)('$name: 自サイトからの JSON 送信は Backend へ渡す', async ({ url, handler }) => {
    await handler(post(url, { 'content-type': 'application/json', origin: 'http://localhost:3000' }))
    expect(fetchMock).toHaveBeenCalled()
  })

  it('セッションを発行しない BFF の経路（面接など）は Origin を見ない', async () => {
    const res = await bffAt('interviews')(post('/api/bff/interviews', { 'content-type': 'application/json' }))
    expect(res.status).toBe(200)
    expect(fetchMock).toHaveBeenCalled()
  })
})
