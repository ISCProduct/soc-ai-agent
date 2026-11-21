import { NextRequest } from 'next/server'
import { GET } from '@/app/api/resume/document/route'

describe('/api/resume/document プロキシ', () => {
  afterEach(() => jest.restoreAllMocks())

  it('Range を上流へ渡し、Accept-Ranges / Content-Range / Content-Length を応答へ写す', async () => {
    const upstream = new Response('abc', {
      status: 206,
      headers: { 'content-type': 'application/pdf', 'accept-ranges': 'bytes', 'content-range': 'bytes 0-2/10' },
    })
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(upstream)
    const req = new NextRequest('http://localhost:3000/api/resume/document?document_id=5', { headers: { range: 'bytes=0-2' } })

    const res = await GET(req)

    expect(fetchMock.mock.calls[0][1]).toMatchObject({ headers: expect.objectContaining({ Range: 'bytes=0-2' }) })
    expect(res.status).toBe(206)
    expect(res.headers.get('accept-ranges')).toBe('bytes')
    expect(res.headers.get('content-range')).toBe('bytes 0-2/10')
    expect(res.headers.get('content-length')).toBe('3')
  })

  it('document_id が数字でなければ Backend へ行かず400', async () => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const res = await GET(new NextRequest('http://localhost:3000/api/resume/document?document_id=abc'))
    expect(res.status).toBe(400)
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
