import { NextRequest } from 'next/server'
import { GET, PUT } from '@/app/api/assessments/[[...path]]/route'

const ctx = (path?: string[]) => ({ params: Promise.resolve({ path }) })

describe('/api/assessments プロキシ', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('認証ヘッダーがなければバックエンドへ行かず401を返す', async () => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const response = await GET(new NextRequest('http://localhost:3000/api/assessments'), ctx())
    expect(response.status).toBe(401)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('一覧はクエリを付けずに /api/assessments へ転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response('{"assessments":[]}', { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const request = new NextRequest('http://localhost:3000/api/assessments?user_id=999', {
      headers: { 'X-User-Token': 'user-jwt' },
    })
    const response = await GET(request, ctx())
    expect(response.status).toBe(200)
    expect(fetchMock.mock.calls[0][0]).toMatch(/\/api\/assessments$/)
  })

  it('回答保存の204を本文なしで返し、本文をそのまま転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(new Response(null, { status: 204 }))
    const request = new NextRequest('http://localhost:3000/api/assessments/sessions/abc/answers/3', {
      method: 'PUT',
      headers: { 'X-User-Token': 'user-jwt' },
      body: JSON.stringify({ value: 4 }),
    })
    const response = await PUT(request, ctx(['sessions', 'abc', 'answers', '3']))
    expect(response.status).toBe(204)
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/assessments\/sessions\/abc\/answers\/3$/),
      expect.objectContaining({ method: 'PUT', body: '{"value":4}' }),
    )
  })

  it('パスの区切りを含む値はエスケープして別の経路へ抜けさせない', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{}', { status: 200 }))
    const request = new NextRequest('http://localhost:3000/api/assessments/x', { headers: { 'X-User-Token': 'user-jwt' } })
    await GET(request, ctx(['sessions', '../../admin']))
    expect(fetchMock.mock.calls[0][0]).toContain('/api/assessments/sessions/..%2F..%2Fadmin')
  })
})
