/** @jest-environment jsdom */

import type { authService as AuthService } from '@/lib/auth'

function storageMock(): Storage {
  const store = new Map<string, string>()
  return {
    get length() {
      return store.size
    },
    clear: () => store.clear(),
    getItem: (k: string) => store.get(k) ?? null,
    key: (i: number) => Array.from(store.keys())[i] ?? null,
    removeItem: (k: string) => {
      store.delete(k)
    },
    setItem: (k: string, v: string) => {
      store.set(k, v)
    },
  }
}

// 管理者トークンは httpOnly Cookie にだけ置き、画面は値を持たない（SOCAIAGENT-683）。
// ensureFreshAdminToken は middleware に Cookie を取り直させるだけ。
describe('authService.ensureFreshAdminToken', () => {
  const originalFetch = global.fetch
  let authService: typeof AuthService

  beforeEach(() => {
    Object.defineProperty(window, 'localStorage', { value: storageMock(), configurable: true })
    Object.defineProperty(window, 'sessionStorage', { value: storageMock(), configurable: true })
    // 確認済みの時刻をモジュールが覚えるので、テストごとに読み直す
    jest.isolateModules(() => {
      authService = require('@/lib/auth').authService
    })
  })

  afterEach(() => {
    jest.restoreAllMocks()
    global.fetch = originalFetch
  })

  it('X-Refresh-Admin-Token を付けてセッションAPIを呼び、トークンの値は受け取らない', async () => {
    const fetchMock = jest.fn(async () => ({ ok: true, json: async () => ({ has_admin_token: true }) }))
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.ensureFreshAdminToken()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/session',
      expect.objectContaining({ headers: { 'X-Refresh-Admin-Token': '1' } }),
    )
    expect(window.localStorage.getItem('token')).toBeNull()
  })

  it('確かめた直後は、画面遷移のたびに問い合わせない', async () => {
    const fetchMock = jest.fn(async () => ({ ok: true, json: async () => ({ has_admin_token: true }) }))
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.ensureFreshAdminToken()
    await authService.ensureFreshAdminToken()

    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('Cookie に管理者トークンが無ければ失敗にする', async () => {
    global.fetch = jest.fn(async () => ({ ok: true, json: async () => ({ has_admin_token: false }) })) as unknown as typeof fetch
    await expect(authService.ensureFreshAdminToken()).rejects.toThrow(/再ログイン/)
  })

  it.each([401, 500])('セッションAPIが %i を返したら失敗にする', async (status) => {
    global.fetch = jest.fn(async () => ({ ok: false, status, json: async () => ({}) })) as unknown as typeof fetch
    await expect(authService.ensureFreshAdminToken()).rejects.toThrow(/再ログイン/)
  })

  it('失敗したあとの再呼び出しは、もう一度セッションAPIを呼ぶ', async () => {
    const fetchMock = jest
      .fn()
      .mockResolvedValueOnce({ ok: false, status: 500, json: async () => ({}) })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ has_admin_token: true }) })
    global.fetch = fetchMock as unknown as typeof fetch

    await expect(authService.ensureFreshAdminToken()).rejects.toThrow()
    await authService.ensureFreshAdminToken()

    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('同時に呼ばれても、セッションAPIは1回だけ呼ぶ', async () => {
    const fetchMock = jest.fn(async () => ({ ok: true, json: async () => ({ has_admin_token: true }) }))
    global.fetch = fetchMock as unknown as typeof fetch

    await Promise.all([authService.ensureFreshAdminToken(), authService.ensureFreshAdminToken()])

    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})

describe('authService.getAdminFetchHeaders', () => {
  it('X-Admin-Token を含めない（middleware が Cookie から付ける）', () => {
    jest.isolateModules(() => {
      const { authService } = require('@/lib/auth')
      expect(authService.getAdminFetchHeaders()).not.toHaveProperty('X-Admin-Token')
    })
  })
})
