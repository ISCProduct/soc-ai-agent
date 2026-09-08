import { NextRequest } from 'next/server'
import { GET } from '@/app/api/resume/status/route'

describe('GET /api/resume/status', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('認証ヘッダーがなければバックエンドへ行かず401を返す', async () => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const request = new NextRequest('http://localhost:3000/api/resume/status')
    const response = await GET(request)
    expect(response.status).toBe(401)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('X-User-Tokenをバックエンドへ転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ has_document: false, latest_score: null, needs_attention: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    const request = new NextRequest('http://localhost:3000/api/resume/status', {
      headers: { 'X-User-Token': 'user-jwt', 'X-User-ID': '7' },
    })
    const response = await GET(request)
    expect(response.status).toBe(200)
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/resume\/status$/),
      expect.objectContaining({
        cache: 'no-store',
        headers: expect.objectContaining({ 'X-User-Token': 'user-jwt' }),
      }),
    )
  })

  // クライアントが投げたクエリがバックエンドへ届かないこと（他人のIDを覗けないため）。
  it('クエリパラメータをバックエンドへ引き継がない', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const request = new NextRequest('http://localhost:3000/api/resume/status?user_id=999', {
      headers: { 'X-User-Token': 'user-jwt' },
    })
    await GET(request)
    expect(fetchMock.mock.calls[0][0]).not.toContain('user_id')
  })

  it('バックエンド接続失敗時もクラッシュしない', async () => {
    jest.spyOn(global, 'fetch').mockRejectedValue(new Error('ECONNREFUSED'))
    jest.spyOn(console, 'error').mockImplementation(() => {})
    const request = new NextRequest('http://localhost:3000/api/resume/status', {
      headers: { 'X-User-Token': 'user-jwt' },
    })
    const response = await GET(request)
    expect(response.status).toBeGreaterThanOrEqual(500)
  })
})
