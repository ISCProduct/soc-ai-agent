import { NextRequest } from 'next/server'
import { GET } from '@/app/api/companies/route'

describe('GET /api/companies', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('クエリをエンコードしてバックエンドへ転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ companies: [{ id: 1 }], total: 1 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )

    const request = new NextRequest('http://localhost:3000/api/companies?limit=5&offset=10&industry=IT/AI&name=A B&tech=C%2B%2B')
    const response = await GET(request)
    const data = await response.json()

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/companies\?limit=5&offset=10&industry=IT%2FAI&name=A%20B&tech=C%2B%2B$/),
      expect.objectContaining({ method: 'GET' }),
    )
    expect(response.status).toBe(200)
    expect(data).toEqual({ companies: [{ id: 1 }], total: 1 })
  })

  it('バックエンドエラー時に同じステータスで返す', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response('backend failed', { status: 502, statusText: 'Bad Gateway' }),
    )

    const request = new NextRequest('http://localhost:3000/api/companies')
    const response = await GET(request)
    const data = await response.json()

    expect(response.status).toBe(502)
    expect(data).toEqual({ error: 'Failed to fetch companies from backend' })
  })
})
