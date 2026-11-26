import { NextRequest } from 'next/server'
import { GET, POST } from '@/app/api/bff/[...path]/route'

// 画面が Backend を直接呼んでいた学生向けAPIの中継（SOCAIAGENT-683）。
// X-User-Token は middleware が Cookie から付けたものを引き継ぎ、画面へはトークンを返さない。
const ctx = (path: string) => ({ params: Promise.resolve({ path: path.split('/') }) })

describe('/api/bff/[...path]', () => {
  let fetchMock: jest.Mock
  beforeEach(() => {
    fetchMock = jest.fn().mockResolvedValue(new Response('{"ok":true}', { status: 200, headers: { 'Content-Type': 'application/json' } }))
    global.fetch = fetchMock
  })

  function backendCall() {
    const [url, init] = fetchMock.mock.calls[0]
    return { url: String(url), init: init as RequestInit, headers: (init as RequestInit).headers as Record<string, string> }
  }

  it.each(['admin/users', 'user/profile-secret', 'interviewsX', 'auth/refresh'])('許可リストに無いパス %s は 404 で、Backend に送らない', async (path) => {
    const response = await GET(new NextRequest(`http://localhost:3000/api/bff/${path}`), ctx(path))
    expect(response.status).toBe(404)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('.. を含むパスは 404', async () => {
    const response = await GET(new NextRequest('http://localhost:3000/api/bff/interviews'), ctx('interviews/../admin/users'))
    expect(response.status).toBe(404)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('middleware が付けた X-User-Token とクエリを Backend に引き継ぐ', async () => {
    const request = new NextRequest('http://localhost:3000/api/bff/interviews/trend?limit=5', {
      headers: { 'X-User-ID': '7', 'X-User-Token': 'cookie-jwt', 'X-Request-ID': 'req-1' },
    })
    await GET(request, ctx('interviews/trend'))
    const { url, headers } = backendCall()
    expect(url).toMatch(/\/api\/interviews\/trend\?limit=5$/)
    expect(headers['X-User-Token']).toBe('cookie-jwt')
    expect(headers['X-Request-ID']).toBe('req-1')
  })

  it('面接の multipart の本文と Content-Type をそのまま渡し、応答の本文も流す', async () => {
    fetchMock.mockResolvedValue(new Response('--b\r\naudio', { status: 200, headers: { 'Content-Type': 'multipart/mixed; boundary=b' } }))
    const form = new FormData()
    form.append('audio', new Blob(['wav-bytes']), 'a.wav')
    const outgoing = new Request('http://localhost:3000/x', { method: 'POST', body: form })
    const request = new NextRequest('http://localhost:3000/api/bff/interviews/5/turn', {
      method: 'POST',
      headers: { 'content-type': outgoing.headers.get('content-type') ?? '', 'X-User-Token': 'cookie-jwt' },
      body: await outgoing.arrayBuffer(),
    })
    const response = await POST(request, ctx('interviews/5/turn'))
    const { headers, init } = backendCall()
    expect(headers['Content-Type']).toMatch(/^multipart\/form-data; boundary=/)
    expect(new TextDecoder().decode(init.body as ArrayBuffer)).toContain('wav-bytes')
    expect(response.headers.get('Content-Type')).toBe('multipart/mixed; boundary=b')
    await expect(response.text()).resolves.toBe('--b\r\naudio')
  })

  it('auth/user の応答からトークンを取り除く', async () => {
    fetchMock.mockResolvedValue(Response.json({ user_id: 7, email: 'a@example.com', token: 'admin-jwt', user_token: 'jwt' }))
    const response = await GET(new NextRequest('http://localhost:3000/api/bff/auth/user', { headers: { 'X-User-Token': 'cookie-jwt' } }), ctx('auth/user'))
    const body = await response.json()
    expect(body).toEqual({ user_id: 7, email: 'a@example.com' })
  })

  describe('ログイン・本登録・ゲスト作成', () => {
    const issued = { user_id: 7, email: 'a@example.com', name: '学生', user_token: 'new-jwt', refresh_token: 'new-refresh' }

    it.each(['auth/login', 'auth/register', 'auth/guest'])('%s はセッション Cookie を張り、画面へはトークンを返さない', async (path) => {
      fetchMock.mockResolvedValue(Response.json(issued))
      const request = new NextRequest(`http://localhost:3000/api/bff/${path}`, {
        method: 'POST',
        headers: { 'content-type': 'application/json', origin: 'http://localhost:3000', cookie: 'admin_token=previous-admin' },
        body: JSON.stringify({ email: 'a@example.com', password: 'x' }),
      })
      const response = await POST(request, ctx(path))
      const setCookie = response.headers.get('set-cookie') ?? ''
      expect(setCookie).toContain('user_token=new-jwt')
      expect(setCookie).toContain('refresh_token=new-refresh')
      expect(setCookie).toMatch(/admin_token=;/)
      const body = await response.json()
      expect(body).not.toHaveProperty('user_token')
      expect(body).not.toHaveProperty('refresh_token')
      expect(body.user_id).toBe(7)
    })

    it('管理者のログインは admin_token も Cookie に張る', async () => {
      fetchMock.mockResolvedValue(Response.json({ ...issued, token: 'admin-jwt' }))
      const request = new NextRequest('http://localhost:3000/api/bff/auth/login', { method: 'POST', headers: { 'content-type': 'application/json', origin: 'http://localhost:3000' }, body: '{}' })
      const response = await POST(request, ctx('auth/login'))
      expect(response.headers.get('set-cookie') ?? '').toContain('admin_token=admin-jwt')
      expect(await response.json()).not.toHaveProperty('token')
    })

    it('ログイン失敗（403 など）はそのまま返し、Cookie は張らない', async () => {
      fetchMock.mockResolvedValue(Response.json({ error: 'email_not_verified' }, { status: 403 }))
      const request = new NextRequest('http://localhost:3000/api/bff/auth/login', { method: 'POST', headers: { 'content-type': 'application/json', origin: 'http://localhost:3000' }, body: '{}' })
      const response = await POST(request, ctx('auth/login'))
      expect(response.status).toBe(403)
      expect(response.headers.get('set-cookie')).toBeNull()
      expect(await response.json()).toEqual({ error: 'email_not_verified' })
    })

    it('ログインには今のセッションのトークンを渡さない', async () => {
      fetchMock.mockResolvedValue(Response.json(issued))
      const request = new NextRequest('http://localhost:3000/api/bff/auth/login', {
        method: 'POST',
        headers: { 'content-type': 'application/json', origin: 'http://localhost:3000', 'X-User-ID': '1', 'X-User-Token': 'current-jwt' },
        body: '{}',
      })
      await POST(request, ctx('auth/login'))
      expect(backendCall().headers['X-User-Token']).toBeUndefined()
      expect(backendCall().headers['X-User-ID']).toBeUndefined()
    })

    it.each([
      [true, 'current-jwt'],
      [false, undefined],
    ])('本登録は promote_guest=%s のときだけゲストのセッションを渡す', async (promote, expected) => {
      fetchMock.mockResolvedValue(Response.json(issued))
      const request = new NextRequest('http://localhost:3000/api/bff/auth/register', {
        method: 'POST',
        headers: { 'content-type': 'application/json', origin: 'http://localhost:3000', 'X-User-Token': 'current-jwt' },
        body: JSON.stringify({ email: 'a@example.com', promote_guest: promote }),
      })
      await POST(request, ctx('auth/register'))
      expect(backendCall().headers['X-User-Token']).toBe(expected)
    })
  })

  it('Backend に届かなければ 502 系のエラーを返す', async () => {
    fetchMock.mockRejectedValue(new Error('down'))
    const response = await GET(new NextRequest('http://localhost:3000/api/bff/interviews'), ctx('interviews'))
    expect(response.status).toBeGreaterThanOrEqual(500)
  })
})
