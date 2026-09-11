import { NextRequest } from 'next/server'
import { PATCH } from '@/app/api/admin/companies/[id]/company-users/[userID]/route'

const ROUTE_PARAMS = { params: Promise.resolve({ id: '10', userID: '1' }) }

const disabledUser = {
  id: 1,
  company_id: 10,
  email: 'hr@example.com',
  disabled: true,
  disabled_at: '2026-09-08T00:00:00Z',
}

function patchRequest(headers: Record<string, string>, body: unknown): NextRequest {
  return new NextRequest('http://localhost:3000/api/admin/companies/10/company-users/1', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...headers },
    body: JSON.stringify(body),
  })
}

function mockBackend(status: number, body: unknown) {
  return jest.spyOn(global, 'fetch').mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

describe('PATCH /api/admin/companies/[id]/company-users/[userID]', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('管理者認証ヘッダーをBackendへ転送する', async () => {
    const fetchMock = mockBackend(200, disabledUser)

    const response = await PATCH(
      patchRequest({ 'X-Admin-Email': 'admin@example.com', 'X-Admin-Token': 'admin-token' }, { disabled: true }),
      ROUTE_PARAMS,
    )

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toMatch(/\/api\/admin\/companies\/10\/company-users\/1$/)
    expect(init?.method).toBe('PATCH')
    expect(init?.headers).toEqual({
      'Content-Type': 'application/json',
      'X-Admin-Email': 'admin@example.com',
      'X-Admin-Token': 'admin-token',
    })
    expect(response.status).toBe(200)
    await expect(response.json()).resolves.toEqual(disabledUser)
  })

  it('リクエストボディをそのまま転送する（再有効化も同じ経路）', async () => {
    const fetchMock = mockBackend(200, { ...disabledUser, disabled: false, disabled_at: null })

    await PATCH(
      patchRequest({ 'X-Admin-Email': 'admin@example.com', 'X-Admin-Token': 'admin-token' }, { disabled: false }),
      ROUTE_PARAMS,
    )

    expect(fetchMock.mock.calls[0][1]?.body).toBe(JSON.stringify({ disabled: false }))
  })

  it('認証ヘッダーが無ければ資格情報を捏造せず、Backendの401をそのまま返す', async () => {
    // GET/POST と同じく BFF では判定せず空ヘッダーで転送し、Backend の 401 を透過させる
    const fetchMock = mockBackend(401, { error: 'unauthorized' })

    const response = await PATCH(patchRequest({}, { disabled: true }), ROUTE_PARAMS)

    expect(fetchMock.mock.calls[0][1]?.headers).toEqual({
      'Content-Type': 'application/json',
      'X-Admin-Email': '',
      'X-Admin-Token': '',
    })
    expect(response.status).toBe(401)
    await expect(response.json()).resolves.toEqual({ error: 'unauthorized' })
  })

  it('他社ユーザーIDの404もそのまま返す', async () => {
    mockBackend(404, { error: 'not found' })

    const response = await PATCH(
      patchRequest({ 'X-Admin-Email': 'admin@example.com', 'X-Admin-Token': 'admin-token' }, { disabled: true }),
      ROUTE_PARAMS,
    )

    expect(response.status).toBe(404)
  })
})
