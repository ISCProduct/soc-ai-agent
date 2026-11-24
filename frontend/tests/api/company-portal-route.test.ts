import { NextRequest } from 'next/server'
import { GET, POST } from '@/app/api/company-portal/[...path]/route'

const ctx = (path: string[]) => ({ params: Promise.resolve({ path }) })

describe('/api/company-portal プロキシ', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('パス要素をエンコードし、クエリを引き継いで転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{"ok":true}', { status: 200 }))
    const request = new NextRequest('http://localhost:3000/api/company-portal/x?page=2', {
      headers: { 'X-Company-User-Token': 'company-jwt' },
    })
    const response = await GET(request, ctx(['students', '日本 語']))
    expect(response.status).toBe(200)
    expect(fetchMock.mock.calls[0][0]).toMatch(
      new RegExp(`/api/company-portal/students/${encodeURIComponent('日本 語')}\\?page=2$`),
    )
  })

  it.each([['..'], ['%2e%2e'], ['a%2Fb'], ['']])('不正なパス要素 %p は Backend へ行かず400', async (bad) => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const response = await GET(new NextRequest('http://localhost:3000/api/company-portal/x'), ctx(['jobs', bad]))
    expect(response.status).toBe(400)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('Backend に繋がらないときは {error, status} の500を返す', async () => {
    jest.spyOn(global, 'fetch').mockRejectedValue(new TypeError('fetch failed'))
    jest.spyOn(console, 'error').mockImplementation(() => {})
    const response = await GET(new NextRequest('http://localhost:3000/api/company-portal/jobs'), ctx(['jobs']))
    expect(response.status).toBe(500)
    expect(await response.json()).toMatchObject({ error: expect.any(String), status: 500 })
  })

  it('X-Request-ID と X-Tenant-Slug を引き継ぎ、利用者用の認証ヘッダーは渡さない', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{}', { status: 201 }))
    const request = new NextRequest('http://localhost:3000/api/company-portal/jobs', {
      method: 'POST',
      headers: {
        'X-Company-User-Token': 'company-jwt',
        'X-Request-ID': 'req-1',
        'X-Tenant-Slug': 'gakuen',
        'X-User-Token': 'student-jwt',
        'Content-Type': 'application/json',
      },
      body: '{"title":"t"}',
    })
    const response = await POST(request, ctx(['jobs']))
    expect(response.status).toBe(201)
    const init = fetchMock.mock.calls[0][1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers).toMatchObject({
      'X-Company-User-Token': 'company-jwt',
      'X-Request-ID': 'req-1',
      'X-Tenant-Slug': 'gakuen',
      'Content-Type': 'application/json',
    })
    expect(headers['X-User-Token']).toBeUndefined()
    expect(init.body).toBe('{"title":"t"}')
  })
})
