import { NextRequest } from 'next/server'
import { GET } from '@/app/api/user/weight-scores/route'

describe('GET /api/user/weight-scores', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('session_id がなければバックエンドへ行かず400を返す', async () => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const request = new NextRequest('http://localhost:3000/api/user/weight-scores')
    const response = await GET(request)
    expect(response.status).toBe(400)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  // このルートは認証ヘッダーを転送しておらず、Backend の /api/user/profile は
  // EchoUserAuth で 401 を返していた。ルートは !res.ok を空配列に潰すため、
  // 履歴書の前後スコア比較と面接のスコア表示が無言で動かなくなっていた。
  it('X-User-Token をバックエンドへ転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ weight_scores: [{ category: 'technical_orientation', score: 72 }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    const request = new NextRequest('http://localhost:3000/api/user/weight-scores?session_id=sess-1', {
      headers: { 'X-User-Token': 'user-jwt', 'X-User-ID': '7' },
    })
    const response = await GET(request)
    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toEqual({
      weight_scores: [{ category: 'technical_orientation', score: 72 }],
    })
    expect(fetchMock).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({ 'X-User-Token': 'user-jwt' }),
      }),
    )
  })

  // 対象ユーザーは X-User-Token から決まる。クエリの user_id を転送すると
  // 他人のIDを指定する余地が生まれ、URL とアクセスログにも user_id が残る。
  it('user_id クエリをバックエンドへ引き継がない', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const request = new NextRequest(
      'http://localhost:3000/api/user/weight-scores?user_id=999&session_id=sess-1',
      { headers: { 'X-User-Token': 'user-jwt' } },
    )
    await GET(request)
    const calledUrl = String(fetchMock.mock.calls[0][0])
    expect(calledUrl).not.toContain('user_id')
    expect(calledUrl).toContain('session_id=sess-1')
  })

  it('バックエンドが401なら空配列で返し画面を壊さない', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(new Response('Unauthorized', { status: 401 }))
    const request = new NextRequest('http://localhost:3000/api/user/weight-scores?session_id=sess-1')
    const response = await GET(request)
    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toEqual({ weight_scores: [] })
  })
})
