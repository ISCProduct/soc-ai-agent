import { test, expect } from '@playwright/test'

/**
 * リクエストID伝播 (#1188)
 *
 * middleware.ts が入口でリクエストIDを採番し、レスポンスヘッダーで返すこと。
 * ここが起点なので、これが欠けると BE / RAG のログを突き合わせられない。
 */
test.describe('リクエストID', () => {
  test('ページレスポンスに X-Request-ID が付与される', async ({ request }) => {
    const res = await request.get('/')
    expect(res.status()).toBeLessThan(400)
    const requestId = res.headers()['x-request-id']
    expect(requestId).toBeTruthy()
    expect(requestId).toMatch(/^[A-Za-z0-9_-]{1,64}$/)
  })

  test('クライアント指定のIDはそのまま引き継がれる', async ({ request }) => {
    const res = await request.get('/', { headers: { 'X-Request-ID': 'e2e-trace-001' } })
    expect(res.headers()['x-request-id']).toBe('e2e-trace-001')
  })

  test('不正な形式のIDは採番し直される', async ({ request }) => {
    const res = await request.get('/', { headers: { 'X-Request-ID': 'bad id with spaces' } })
    const requestId = res.headers()['x-request-id']
    expect(requestId).not.toBe('bad id with spaces')
    expect(requestId).toMatch(/^[A-Za-z0-9_-]{1,64}$/)
  })

  test('リクエストごとに異なるIDが採番される', async ({ request }) => {
    const [a, b] = await Promise.all([request.get('/'), request.get('/')])
    expect(a.headers()['x-request-id']).not.toBe(b.headers()['x-request-id'])
  })
})
