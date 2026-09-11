import { NextRequest } from 'next/server'
import { POST } from '@/app/api/resume/review/route'

describe('POST /api/resume/review', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('document_id が無い場合は 400 を返す', async () => {
    const request = new NextRequest('http://localhost:3000/api/resume/review', {
      method: 'POST',
      body: JSON.stringify({ company_name: 'テスト株式会社' }),
    })
    const response = await POST(request)
    const data = await response.json()

    expect(response.status).toBe(400)
    expect(data).toEqual({ error: 'document_id is required' })
  })

  it('バックエンドのJSONレスポンスをそのまま返す', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ review_id: 10, score: 88 }), { status: 201 }),
    )

    const request = new NextRequest('http://localhost:3000/api/resume/review?document_id=99', {
      method: 'POST',
      body: JSON.stringify({ company_name: 'テスト株式会社' }),
    })
    const response = await POST(request)
    const data = await response.json()

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/resume\/review\?document_id=99$/),
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ 'Content-Type': 'application/json' }),
      }),
    )
    expect(response.status).toBe(201)
    expect(data).toEqual({ review_id: 10, score: 88 })
  })

  it('バックエンドが非JSON文字列を返した場合は message に変換する', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(new Response('ok-text', { status: 200 }))

    const request = new NextRequest('http://localhost:3000/api/resume/review?document_id=42', {
      method: 'POST',
      body: '',
    })
    const response = await POST(request)
    const data = await response.json()

    expect(response.status).toBe(200)
    expect(data).toEqual({ message: 'ok-text' })
  })
})
