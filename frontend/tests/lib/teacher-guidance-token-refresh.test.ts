/** @jest-environment jsdom */

import { fetchActiveGuidances, dismissGuidance } from '@/lib/teacher-guidance'

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({}),
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

// 以前は Backend を直接呼んでいて middleware の Cookie 自動リフレッシュが効かず、
// ログインから1時間でここだけ401になった。中継ルート（/api/bff）を通せば
// middleware が Cookie から付け直すので、画面がトークンを同期する必要はない（SOCAIAGENT-683）。
describe('teacher-guidance は中継ルートを通す', () => {
  it('一覧取得は /api/bff/user/guidances を呼ぶ', async () => {
    const fetchMock = mockFetch({ guidances: [] })
    await fetchActiveGuidances()
    expect(fetchMock.mock.calls[0][0]).toBe('/api/bff/user/guidances')
  })

  it('閉じる操作も /api/bff を通す', async () => {
    const fetchMock = mockFetch(undefined, 204)
    await dismissGuidance(1)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/bff/user/guidances/1/dismiss')
  })
})
