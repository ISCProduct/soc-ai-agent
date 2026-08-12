import { buildProxyJsonResponse } from '@/lib/api-proxy'

describe('buildProxyJsonResponse HTML 502', () => {
  it('HTML を error/detail に載せない', async () => {
    const html = '<html><head><title>502 Bad Gateway</title></head><body>502</body></html>'
    const res = await buildProxyJsonResponse(new Response(html, { status: 502 }))
    const data = await res.json() as { error: string; detail?: string; status: number }

    expect(res.status).toBe(502)
    expect(data.status).toBe(502)
    expect(data.error).not.toMatch(/<html|Bad Gateway/)
    expect(data.detail ?? '').not.toMatch(/<html|Bad Gateway/)
  })
})
