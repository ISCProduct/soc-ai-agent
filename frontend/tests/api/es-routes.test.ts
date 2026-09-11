import { NextRequest } from 'next/server'
import { POST as rewritePOST } from '@/app/api/es/rewrite/route'
import { POST as reviewPOST } from '@/app/api/es/review/route'

describe('POST /api/es/rewrite', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('バックエンドのJSONレスポンスをそのまま返す', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ rewritten_text: '改稿文' }), { status: 200 }),
    )

    const request = new NextRequest('http://localhost:3000/api/es/rewrite', {
      method: 'POST',
      body: JSON.stringify({ original_text: '原文' }),
    })
    const response = await rewritePOST(request)
    const data = await response.json()

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/es\/rewrite$/),
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ 'Content-Type': 'application/json' }),
      }),
    )
    expect(response.status).toBe(200)
    expect(data).toEqual({ rewritten_text: '改稿文' })
  })

  it('認証ヘッダーがあればバックエンドへ転送する(#1015)', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ rewritten_text: '改稿文' }), { status: 200 }),
    )

    const request = new NextRequest('http://localhost:3000/api/es/rewrite', {
      method: 'POST',
      headers: { 'X-User-ID': 'u1', 'X-User-Token': 'tok' },
      body: JSON.stringify({ original_text: '原文' }),
    })
    await rewritePOST(request)

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/es\/rewrite$/),
      expect.objectContaining({
        headers: expect.objectContaining({ 'X-User-ID': 'u1', 'X-User-Token': 'tok' }),
      }),
    )
  })

  it('認証ヘッダーが無くても従来通りバックエンドへリクエストできる（未ログイン許容を維持）', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ rewritten_text: '改稿文' }), { status: 200 }),
    )

    const request = new NextRequest('http://localhost:3000/api/es/rewrite', {
      method: 'POST',
      body: JSON.stringify({ original_text: '原文' }),
    })
    const response = await rewritePOST(request)

    expect(fetchMock).toHaveBeenCalled()
    expect(response.status).toBe(200)
  })
})

describe('POST /api/es/review', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('バックエンドのJSONレスポンスをそのまま返す', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ specificity_score: 80 }), { status: 200 }),
    )

    const request = new NextRequest('http://localhost:3000/api/es/review', {
      method: 'POST',
      body: JSON.stringify({ es_text: 'ES本文' }),
    })
    const response = await reviewPOST(request)
    const data = await response.json()

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/es\/review$/),
      expect.objectContaining({ method: 'POST' }),
    )
    expect(response.status).toBe(200)
    expect(data).toEqual({ specificity_score: 80 })
  })
})
