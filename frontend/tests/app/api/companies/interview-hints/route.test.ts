import { NextRequest } from 'next/server'
import { POST } from '@/app/api/companies/interview-hints/route'

describe('企業別面接ヒントAPI', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.clearAllMocks()
  })

  it('RAGが失敗した場合はエラーを返し、空の成功レスポンスにしない', async () => {
    global.fetch = jest.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ brief: '企業概要' }),
      } as Response)
      .mockResolvedValueOnce({
        ok: false,
        status: 503,
      } as Response)

    const request = new NextRequest('http://localhost/api/companies/interview-hints', {
      method: 'POST',
      body: JSON.stringify({ company_name: 'テスト株式会社', position: 'エンジニア' }),
      headers: { 'Content-Type': 'application/json' },
    })
    const response = await POST(request)

    expect(response.status).toBe(502)
    await expect(response.json()).resolves.toEqual({
      error: '企業の面接傾向を取得できませんでした',
    })
  })

  it('企業名が空の場合は入力エラーを返す', async () => {
    global.fetch = jest.fn()
    const request = new NextRequest('http://localhost/api/companies/interview-hints', {
      method: 'POST',
      body: JSON.stringify({ company_name: '  ' }),
      headers: { 'Content-Type': 'application/json' },
    })

    const response = await POST(request)

    expect(response.status).toBe(400)
    expect(global.fetch).not.toHaveBeenCalled()
  })
})
