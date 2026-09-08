import { NextRequest } from 'next/server'
import { GET } from '@/app/api/admin/teacher/students/tendency-analysis/route'

describe('GET /api/admin/teacher/students/tendency-analysis', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('管理者認証ヘッダとクエリパラメータをそのままバックエンドへ転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ students: [], total: 0, limit: 25, offset: 0 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const request = new NextRequest(
      'http://localhost:3000/api/admin/teacher/students/tendency-analysis?limit=10&offset=20&q=%E5%B1%B1%E7%94%B0&school_id=3',
      { headers: { 'X-Admin-Email': 'admin@example.com', 'X-Admin-Token': 'admin-token' } },
    )
    const response = await GET(request)
    const data = await response.json()

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0]
    expect(String(url)).toContain('/api/admin/teacher/students/tendency-analysis?')
    const forwarded = new URL(String(url)).searchParams
    expect(forwarded.get('limit')).toBe('10')
    expect(forwarded.get('offset')).toBe('20')
    expect(forwarded.get('q')).toBe('山田')
    expect(forwarded.get('school_id')).toBe('3')
    expect(init?.headers).toEqual({
      'X-Admin-Email': 'admin@example.com',
      'X-Admin-Token': 'admin-token',
    })
    expect(response.status).toBe(200)
    expect(data).toEqual({ students: [], total: 0, limit: 25, offset: 0 })
  })

  it('クエリが無ければ ? を付けずに転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ students: [], total: 0 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const request = new NextRequest('http://localhost:3000/api/admin/teacher/students/tendency-analysis')
    await GET(request)

    expect(String(fetchMock.mock.calls[0][0])).toMatch(/\/api\/admin\/teacher\/students\/tendency-analysis$/)
  })

  it('バックエンドのエラーステータスをそのまま返す(403など)', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: '担当校の指定が必要です' }), {
        status: 403,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const request = new NextRequest('http://localhost:3000/api/admin/teacher/students/tendency-analysis')
    const response = await GET(request)

    expect(response.status).toBe(403)
    expect(await response.json()).toEqual({ error: '担当校の指定が必要です' })
  })
})
