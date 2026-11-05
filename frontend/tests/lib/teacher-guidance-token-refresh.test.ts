/** @jest-environment jsdom */

import { fetchActiveGuidances, dismissGuidance } from '@/lib/teacher-guidance'

const ensureFreshUserToken = jest.fn().mockResolvedValue(undefined)

jest.mock('@/lib/auth', () => ({
  authService: {
    ensureFreshUserToken: (...args: unknown[]) => ensureFreshUserToken(...args),
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

// Backend 直叩きは middleware の Cookie 自動リフレッシュが効かず、localStorage の
// JWT が1時間で失効したまま残る（ログイン画面以外は正常なのにここだけ401になる）。
// 面接・スカウトAPIと同じく、本体のリクエスト前に ensureFreshUserToken() を呼ぶことを固定する。
describe('teacher-guidance トークンリフレッシュ', () => {
  beforeEach(() => {
    ensureFreshUserToken.mockClear()
  })

  it('一覧取得の前にトークンを更新する', async () => {
    mockFetch({ guidances: [] })

    await fetchActiveGuidances()

    expect(ensureFreshUserToken).toHaveBeenCalledTimes(1)
  })

  it('閉じる操作の前にもトークンを更新する', async () => {
    mockFetch(undefined, 204)

    await dismissGuidance(1)

    expect(ensureFreshUserToken).toHaveBeenCalledTimes(1)
  })
})
