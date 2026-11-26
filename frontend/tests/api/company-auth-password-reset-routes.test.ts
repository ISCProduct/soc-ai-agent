import { NextRequest } from 'next/server'
import { POST as forgotPassword } from '@/app/api/company-auth/forgot-password/route'
import { POST as resetPassword } from '@/app/api/company-auth/reset-password/route'

// 成功時のBackendレスポンス（AcceptInviteと同じAuthResponse形）
const authResponse = {
  company_user_id: 1,
  company_id: 10,
  email: 'hr@example.com',
  name: '採用担当',
  role: 'member',
  token: 'jwt-token',
  refresh_token: 'refresh-token',
}

function jsonRequest(url: string, body: unknown): NextRequest {
  return new NextRequest(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', origin: 'http://localhost:3000' },
    body: JSON.stringify(body),
  })
}

describe('POST /api/company-auth/forgot-password', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('認証ヘッダー無しでもバックエンドへそのまま転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ message: 'パスワード再設定用のメールを送信しました' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json', origin: 'http://localhost:3000' },
      }),
    )

    const response = await forgotPassword(
      jsonRequest('http://localhost:3000/api/company-auth/forgot-password', {
        email: 'hr@example.com',
      }),
    )

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/company-auth\/forgot-password$/),
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ email: 'hr@example.com' }),
      }),
    )
    // 認証ヘッダーを要求しない（未ログインでも呼べる）
    const forwardedHeaders = fetchMock.mock.calls[0][1]?.headers as Record<string, string>
    expect(forwardedHeaders).toEqual({ 'Content-Type': 'application/json' })
    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toEqual({
      message: 'パスワード再設定用のメールを送信しました',
    })
  })

  it('未登録のメールでもバックエンドの200をそのまま返す（存在有無を漏らさない）', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ message: 'パスワード再設定用のメールを送信しました' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json', origin: 'http://localhost:3000' },
      }),
    )

    const response = await forgotPassword(
      jsonRequest('http://localhost:3000/api/company-auth/forgot-password', {
        email: 'unknown@example.com',
      }),
    )

    expect(response.status).toBe(200)
    expect(response.headers.getSetCookie()).toEqual([])
  })

  it('レート制限(429)は同じステータスで返す', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'too many requests' }), {
        status: 429,
        headers: { 'Content-Type': 'application/json', origin: 'http://localhost:3000' },
      }),
    )

    const response = await forgotPassword(
      jsonRequest('http://localhost:3000/api/company-auth/forgot-password', {
        email: 'hr@example.com',
      }),
    )

    expect(response.status).toBe(429)
  })
})

describe('POST /api/company-auth/reset-password', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('認証ヘッダー無しでバックエンドへ送り、成功したら Cookie を張ってトークンを返さない', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(authResponse), {
        status: 200,
        headers: { 'Content-Type': 'application/json', origin: 'http://localhost:3000' },
      }),
    )

    const response = await resetPassword(
      jsonRequest('http://localhost:3000/api/company-auth/reset-password', {
        token: 'reset-token',
        password: 'newpassword123',
      }),
    )

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/company-auth\/reset-password$/),
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ token: 'reset-token', password: 'newpassword123' }),
      }),
    )
    const forwardedHeaders = fetchMock.mock.calls[0][1]?.headers as Record<string, string>
    expect(forwardedHeaders).toEqual({ 'Content-Type': 'application/json' })

    expect(response.status).toBe(200)
    // ログイン・登録・招待の受け入れと同じく、このルートで httpOnly Cookie を張り、
    // トークンは画面へ返さない（SOCAIAGENT-683）
    const { token: _token, refresh_token: _refresh, ...withoutTokens } = authResponse as Record<string, unknown>
    await expect(response.json()).resolves.toEqual(withoutTokens)
    const setCookie = response.headers.getSetCookie().join('\n')
    expect(setCookie).toContain('company_user_token=')
    expect(setCookie).toMatch(/HttpOnly/i)
  })

  it('トークン無効(400)は同じステータスでそのまま返す', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'invalid or expired token' }), {
        status: 400,
        headers: { 'Content-Type': 'application/json', origin: 'http://localhost:3000' },
      }),
    )

    const response = await resetPassword(
      jsonRequest('http://localhost:3000/api/company-auth/reset-password', {
        token: 'expired',
        password: 'newpassword123',
      }),
    )

    expect(response.status).toBe(400)
    expect(response.headers.getSetCookie()).toEqual([])
    await expect(response.json()).resolves.toEqual({ error: 'invalid or expired token' })
  })
})
