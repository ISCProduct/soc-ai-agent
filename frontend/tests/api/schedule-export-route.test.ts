import { NextRequest } from 'next/server'
import { GET } from '@/app/api/schedule/export/route'

// #1012: ICSエクスポートがuser_idクエリのみでBackendのJWT認証(#983)を通せず
// 401になっていた回帰を防ぐ。
describe('GET /api/schedule/export', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('認証ヘッダーがなければBackendへ問い合わせず401を返す', async () => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const request = new NextRequest('http://localhost:3000/api/schedule/export')
    const response = await GET(request)

    expect(response.status).toBe(401)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('X-User-Tokenをバックエンドへ転送し、認証付きでicsを取得できる', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response('BEGIN:VCALENDAR\nEND:VCALENDAR', {
        status: 200,
        headers: { 'Content-Type': 'text/calendar' },
      }),
    )
    const request = new NextRequest('http://localhost:3000/api/schedule/export', {
      headers: { 'X-User-Token': 'user-jwt' },
    })
    const response = await GET(request)
    const body = await response.text()

    expect(response.status).toBe(200)
    expect(response.headers.get('Content-Type')).toContain('text/calendar')
    expect(body).toContain('BEGIN:VCALENDAR')
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/schedule\/export\/ics$/),
      expect.objectContaining({
        headers: expect.objectContaining({ 'X-User-Token': 'user-jwt' }),
      }),
    )
  })
})
