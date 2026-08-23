import { NextRequest } from 'next/server'
import { GET, PUT, DELETE } from '@/app/api/schedule/[id]/route'

// #1012: 個別GET/PUT/DELETEがuser_idクエリのみでBackendのJWT認証(#983)を通せず
// 401になっていた回帰を防ぐ。一覧(/api/schedule)と同じextractUserAuthHeadersで
// X-User-Tokenを転送し、未認証時はBackendへ問い合わせず401を返すことを検証する。
const params = Promise.resolve({ id: '1' })

describe('/api/schedule/[id]', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  describe('GET', () => {
    it('認証ヘッダーがなければBackendへ問い合わせず401を返す', async () => {
      const fetchMock = jest.spyOn(global, 'fetch')
      const request = new NextRequest('http://localhost:3000/api/schedule/1')
      const response = await GET(request, { params })

      expect(response.status).toBe(401)
      expect(fetchMock).not.toHaveBeenCalled()
    })

    it('X-User-Tokenをバックエンドへ転送し、user_idクエリなしで取得できる', async () => {
      const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
        new Response(JSON.stringify({ id: 1, company_name: 'テスト株式会社' }), { status: 200 }),
      )
      const request = new NextRequest('http://localhost:3000/api/schedule/1', {
        headers: { 'X-User-Token': 'user-jwt', 'X-User-ID': '5' },
      })
      const response = await GET(request, { params })
      const data = await response.json()

      expect(response.status).toBe(200)
      expect(data).toEqual({ id: 1, company_name: 'テスト株式会社' })
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringMatching(/\/api\/schedule\/1$/),
        expect.objectContaining({
          headers: expect.objectContaining({ 'X-User-Token': 'user-jwt', 'X-User-ID': '5' }),
        }),
      )
    })
  })

  describe('PUT', () => {
    it('認証ヘッダーがなければBackendへ問い合わせず401を返す', async () => {
      const fetchMock = jest.spyOn(global, 'fetch')
      const request = new NextRequest('http://localhost:3000/api/schedule/1', {
        method: 'PUT',
        body: JSON.stringify({ stage: '内定' }),
      })
      const response = await PUT(request, { params })

      expect(response.status).toBe(401)
      expect(fetchMock).not.toHaveBeenCalled()
    })

    it('X-User-Tokenをバックエンドへ転送し200で更新できる', async () => {
      const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
        new Response(JSON.stringify({ id: 1, stage: '内定' }), { status: 200 }),
      )
      const request = new NextRequest('http://localhost:3000/api/schedule/1', {
        method: 'PUT',
        headers: { 'X-User-Token': 'user-jwt' },
        body: JSON.stringify({ stage: '内定' }),
      })
      const response = await PUT(request, { params })

      expect(response.status).toBe(200)
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringMatching(/\/api\/schedule\/1$/),
        expect.objectContaining({
          method: 'PUT',
          headers: expect.objectContaining({ 'X-User-Token': 'user-jwt' }),
        }),
      )
    })
  })

  describe('DELETE', () => {
    it('認証ヘッダーがなければBackendへ問い合わせず401を返す', async () => {
      const fetchMock = jest.spyOn(global, 'fetch')
      const request = new NextRequest('http://localhost:3000/api/schedule/1', { method: 'DELETE' })
      const response = await DELETE(request, { params })

      expect(response.status).toBe(401)
      expect(fetchMock).not.toHaveBeenCalled()
    })

    it('X-User-Tokenをバックエンドへ転送し204で削除できる', async () => {
      const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(new Response(null, { status: 204 }))
      const request = new NextRequest('http://localhost:3000/api/schedule/1', {
        method: 'DELETE',
        headers: { 'X-User-Token': 'user-jwt' },
      })
      const response = await DELETE(request, { params })

      expect(response.status).toBe(204)
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringMatching(/\/api\/schedule\/1$/),
        expect.objectContaining({
          method: 'DELETE',
          headers: expect.objectContaining({ 'X-User-Token': 'user-jwt' }),
        }),
      )
    })
  })
})
