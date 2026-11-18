import { NextRequest } from 'next/server'
import { POST } from '@/app/api/resume/review/stream/route'

describe('POST /api/resume/review/stream (SOCAIAGENT-664)', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  const req = () =>
    new NextRequest('http://localhost:3000/api/resume/review/stream?document_id=9', { method: 'POST', body: '{}' })

  it('request.signal を上流の fetch に渡す', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(new Response('data: {}\n\n', { status: 200 }))
    const request = req()
    await POST(request)
    expect(fetchMock.mock.calls[0][1]).toEqual(expect.objectContaining({ signal: request.signal }))
  })

  it('上流が 5xx なら内部の本文を出さず固定文言の 502 を返す', async () => {
    jest.spyOn(console, 'error').mockImplementation(() => undefined)
    jest.spyOn(global, 'fetch').mockResolvedValue(new Response('panic: secret internal detail', { status: 500 }))
    const response = await POST(req())
    expect(response.status).toBe(502)
    const text = await response.text()
    expect(text).not.toContain('secret')
    expect(text).toContain('レビューに失敗しました')
  })

  it('上流への接続に失敗したら固定文言の 502 を返す', async () => {
    jest.spyOn(console, 'error').mockImplementation(() => undefined)
    jest.spyOn(global, 'fetch').mockRejectedValue(new Error('connect ECONNREFUSED 10.0.0.5:8080'))
    const response = await POST(req())
    expect(response.status).toBe(502)
    expect(await response.text()).not.toContain('10.0.0.5')
  })

  it('上流が 4xx ならその本文を返す', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{"error":"forbidden"}', { status: 403 }))
    const response = await POST(req())
    expect(response.status).toBe(403)
  })
})
