import { NextRequest } from 'next/server'
import { proxyAdminBackend } from '@/lib/admin/backend-proxy'
import * as school from '@/app/api/admin/schools/[id]/route'
import * as retention from '@/app/api/admin/schools/[id]/interview-video-retention/route'

jest.mock('@/lib/admin/backend-proxy', () => ({
  ...jest.requireActual('@/lib/admin/backend-proxy'),
  proxyAdminBackend: jest.fn(),
}))

const adminHeaders = { 'X-Admin-Email': 'a@example.com', 'X-Admin-Token': 't' }

describe('学校の面接動画保持期間の admin プロキシ', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    jest.mocked(proxyAdminBackend).mockResolvedValue({ status: 200, text: '{"interview_video_retention_days":30}' } as never)
  })

  it('GET /api/admin/schools/{id} は Backend へ転送し、id をエンコードして管理者ヘッダーを引き継ぐ', async () => {
    const req = new NextRequest('http://localhost/api/admin/schools/1', { headers: adminHeaders })
    const res = await school.GET(req, { params: Promise.resolve({ id: 'a/b' }) })
    const [method, path, options] = jest.mocked(proxyAdminBackend).mock.calls[0]
    expect(method).toBe('GET')
    expect(path).toBe('/api/admin/schools/a%2Fb')
    expect(options?.headers).toMatchObject({ 'X-Admin-Email': 'a@example.com', 'X-Admin-Token': 't' })
    expect(res.status).toBe(200)
    await expect(res.json()).resolves.toEqual({ interview_video_retention_days: 30 })
  })

  it('PUT .../interview-video-retention は本文をそのまま転送し、Backend のエラーも返す', async () => {
    jest.mocked(proxyAdminBackend).mockResolvedValue({ status: 400, text: '{"error":"invalid"}' } as never)
    const req = new NextRequest('http://localhost/api/admin/schools/1/interview-video-retention', {
      method: 'PUT',
      headers: adminHeaders,
      body: '{"days":45}',
    })
    const res = await retention.PUT(req, { params: Promise.resolve({ id: '../2' }) })
    const [method, path, options] = jest.mocked(proxyAdminBackend).mock.calls[0]
    expect(method).toBe('PUT')
    expect(path).toBe('/api/admin/schools/..%2F2/interview-video-retention')
    expect(options?.body).toBe('{"days":45}')
    expect(res.status).toBe(400)
  })

  it('Backend に接続できないときは 503 を返す', async () => {
    jest.mocked(proxyAdminBackend).mockRejectedValue(new Error('connect ECONNREFUSED'))
    const req = new NextRequest('http://localhost/api/admin/schools/1', { headers: adminHeaders })
    const res = await school.GET(req, { params: Promise.resolve({ id: '1' }) })
    expect(res.status).toBe(503)
  })
})
