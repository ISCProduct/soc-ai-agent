/** @jest-environment jsdom */

import { BACKEND_URL } from '@/lib/config'
import { studentScoutService } from '@/lib/scout/api'

jest.mock('@/lib/auth', () => ({
  authService: {
    ensureFreshUserToken: jest.fn().mockResolvedValue(undefined),
    getUserFetchHeaders: () => ({ 'X-User-Token': 'user-jwt' }),
  },
}))

function mockFetch(body: unknown, status = 200) {
  const fetchMock = jest.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  })
  global.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

describe('studentScoutService', () => {
  it('受信一覧は認証ヘッダーと limit/offset を付けて取得する', async () => {
    const fetchMock = mockFetch({ items: [], total: 0, blocked_company_ids: [] })

    await studentScoutService.list({ limit: 30, offset: 30 })

    expect(fetchMock).toHaveBeenCalledWith(
      `${BACKEND_URL}/api/user/scouts?limit=30&offset=30`,
      expect.objectContaining({
        headers: expect.objectContaining({ 'X-User-Token': 'user-jwt' }),
      }),
    )
  })

  it('既読・辞退・ブロックは本番のAPIへ送る', async () => {
    const fetchMock = mockFetch({ id: 4, company_id: 8, status: 'viewed', message: '本文' })

    await studentScoutService.view(4)
    await studentScoutService.decline(4)
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 204,
      json: async () => undefined,
    })
    await studentScoutService.blockCompany(8)

    const urls = fetchMock.mock.calls.map((call) => call[0] as string)
    expect(urls).toEqual([
      `${BACKEND_URL}/api/user/scouts/4/view`,
      `${BACKEND_URL}/api/user/scouts/4/decline`,
      `${BACKEND_URL}/api/user/scout-blocks`,
    ])
    const methods = fetchMock.mock.calls.map((call) => (call[1] as RequestInit).method)
    expect(methods).toEqual(['POST', 'POST', 'POST'])
    const blockBody = JSON.parse((fetchMock.mock.calls[2][1] as RequestInit).body as string)
    expect(blockBody).toEqual({ company_id: 8 })
    for (const call of fetchMock.mock.calls) {
      expect((call[1] as RequestInit).headers).toEqual(
        expect.objectContaining({ 'X-User-Token': 'user-jwt' }),
      )
    }
  })
})
