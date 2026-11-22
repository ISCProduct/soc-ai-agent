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

  it.each(['admin/users', 'user/profile-secret', 'interviewsX', 'auth/refresh', 'auth/login', 'auth/guest'])('許可リストに無いパス %s は 404 で、Backend に送らない', async (path) => {
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

  describe('本登録', () => {
    const issued = { user_id: 7, email: 'a@example.com', name: '学生', user_token: 'new-jwt' }

    it.each([
      [true, 'current-jwt'],
      [false, undefined],
    ])('promote_guest=%s のときだけゲストのセッションを渡す', async (promote, expected) => {
      fetchMock.mockResolvedValue(Response.json(issued))
      const request = new NextRequest('http://localhost:3000/api/bff/auth/register', {
        method: 'POST',
        headers: { 'content-type': 'application/json', 'X-User-Token': 'current-jwt' },
        body: JSON.stringify({ email: 'a@example.com', promote_guest: promote }),
      })
      await POST(request, ctx('auth/register'))
      expect(backendCall().headers['X-User-Token']).toBe(expected)
    })

    it('応答はそのまま返す（Cookie は画面の saveAuth が張る。ログイン経路の見直しは別 PR）', async () => {
      fetchMock.mockResolvedValue(Response.json(issued))
      const request = new NextRequest('http://localhost:3000/api/bff/auth/register', {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: '{}',
      })
      const response = await POST(request, ctx('auth/register'))
      expect(await response.json()).toEqual(issued)
    })
  })

  it('Backend に届かなければ 502 系のエラーを返す', async () => {
    fetchMock.mockRejectedValue(new Error('down'))
    const response = await GET(new NextRequest('http://localhost:3000/api/bff/interviews'), ctx('interviews'))
    expect(response.status).toBeGreaterThanOrEqual(500)
  })
})
