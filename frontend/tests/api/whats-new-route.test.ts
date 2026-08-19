import { NextRequest } from 'next/server'
import { GET } from '@/app/api/whats-new/route'

describe('GET /api/whats-new', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('認証ヘッダーがなければ401を返す', async () => {
    const request = new NextRequest('http://localhost:3000/api/whats-new')
    const response = await GET(request)
    expect(response.status).toBe(401)
  })

  it('X-User-Tokenをバックエンドへ転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify([]), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const request = new NextRequest('http://localhost:3000/api/whats-new', {
      headers: { 'X-User-Token': 'user-jwt' },
    })
    const response = await GET(request)
    expect(response.status).toBe(200)
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/whats-new$/),
      expect.objectContaining({
        headers: expect.objectContaining({ 'X-User-Token': 'user-jwt' }),
      }),
    )
  })
})
