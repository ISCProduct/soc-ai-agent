import { NextRequest } from 'next/server'
import { GET, POST } from '@/app/api/admin/assessment-templates/[[...path]]/route'

const ctx = (path?: string[]) => ({ params: Promise.resolve({ path }) })
const admin = { 'x-admin-email': 'a@example.com', 'x-admin-token': 'tok', 'x-request-id': 'req-1' }

describe('/api/admin/assessment-templates プロキシ', () => {
  afterEach(() => jest.restoreAllMocks())

  it('一覧は school_id だけを転送し、他のクエリは素通しにしない', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{"templates":[]}', { status: 200 }))
    const req = new NextRequest('http://localhost:3000/api/admin/assessment-templates?school_id=10&user_id=999', { headers: admin })
    const res = await GET(req, ctx())
    expect(res.status).toBe(200)
    expect(fetchMock.mock.calls[0][0]).toMatch(/\/api\/admin\/assessment-templates\?school_id=10$/)
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ headers: expect.objectContaining({ 'X-Admin-Email': 'a@example.com', 'X-Admin-Token': 'tok', 'X-Request-ID': 'req-1' }) })
  })

  it('受検状況は :id/students へ転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{}', { status: 200 }))
    await GET(new NextRequest('http://localhost:3000/x?school_id=10', { headers: admin }), ctx(['7', 'students']))
    expect(fetchMock.mock.calls[0][0]).toMatch(/\/api\/admin\/assessment-templates\/7\/students\?school_id=10$/)
  })

  it('作成は本文を転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{}', { status: 201 }))
    const req = new NextRequest('http://localhost:3000/x?school_id=10', { method: 'POST', headers: admin, body: '{"title":"A"}' })
    const res = await POST(req, ctx())
    expect(res.status).toBe(201)
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ method: 'POST', body: '{"title":"A"}' })
  })

  it.each([[['abc', 'students']], [['7']], [['7', 'students', 'x']], [['..', 'students']], [['7', 'answers']]])(
    '許可していない経路 %j は Backend へ行かず404',
    async (path) => {
      const fetchMock = jest.spyOn(global, 'fetch')
      const res = await GET(new NextRequest('http://localhost:3000/x?school_id=10', { headers: admin }), ctx(path))
      expect(res.status).toBe(404)
      expect(fetchMock).not.toHaveBeenCalled()
    },
  )

  it('Backend の403（担当校外）をそのまま返す', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{"message":"school access denied"}', { status: 403 }))
    const res = await GET(new NextRequest('http://localhost:3000/x?school_id=11', { headers: admin }), ctx())
    expect(res.status).toBe(403)
  })

  it('Backend 停止・非JSONでも502/本文つきで返す（throwしない）', async () => {
    jest.spyOn(console, 'error').mockImplementation(() => {})
    jest.spyOn(global, 'fetch').mockRejectedValue(new Error('down'))
    const res = await GET(new NextRequest('http://localhost:3000/x?school_id=10', { headers: admin }), ctx())
    expect(res.status).toBe(502)
  })
})
