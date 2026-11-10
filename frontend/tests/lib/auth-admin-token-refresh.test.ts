/** @jest-environment jsdom */

import { authService } from '@/lib/auth'

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

describe('authService.ensureFreshAdminToken', () => {
  const originalFetch = global.fetch
  const staleToken = () => `${Math.floor(Date.now() / 1000) - 8 * 24 * 60 * 60}.old`

  beforeEach(() => {
    Object.defineProperty(window, 'localStorage', { value: storageMock(), configurable: true })
    Object.defineProperty(window, 'sessionStorage', { value: storageMock(), configurable: true })
  })

  afterEach(() => {
    jest.restoreAllMocks()
    global.fetch = originalFetch
  })

  it('期限が近いトークンはセッションAPIの新しい値で置き換える', async () => {
    const stale = `${Math.floor(Date.now() / 1000) - 8 * 24 * 60 * 60}.old`
    const fresh = `${Math.floor(Date.now() / 1000)}.new`
    window.localStorage.setItem('token', stale)
    const fetchMock = jest.fn(async () => ({
      ok: true,
      json: async () => ({ token: fresh }),
    }))
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.ensureFreshAdminToken()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/session',
      expect.objectContaining({
        headers: { 'X-Refresh-Admin-Token': '1' },
      }),
    )
    expect(authService.getStoredToken()).toBe(fresh)
  })

  it('まだ新しければセッションAPIを呼ばない', async () => {
    window.localStorage.setItem('token', `${Math.floor(Date.now() / 1000)}.current`)
    const fetchMock = jest.fn()
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.ensureFreshAdminToken()

    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('空の応答では古いトークンを残して失敗にする', async () => {
    const stale = `${Math.floor(Date.now() / 1000) - 8 * 24 * 60 * 60}.old`
    window.localStorage.setItem('token', stale)
    global.fetch = jest.fn(async () => ({
      ok: true,
      json: async () => ({}),
    })) as unknown as typeof fetch

    await expect(authService.ensureFreshAdminToken()).rejects.toThrow(/再ログイン/)
    expect(authService.getStoredToken()).toBe(stale)
  })
  it.each([401, 500])('セッションAPIが %i を返したら失敗にし、古いトークンを残す', async (status) => {
    const stale = staleToken()
    window.localStorage.setItem('token', stale)
    global.fetch = jest.fn(async () => ({ ok: false, status, json: async () => ({}) })) as unknown as typeof fetch

    await expect(authService.ensureFreshAdminToken()).rejects.toThrow(/再ログイン/)
    expect(authService.getStoredToken()).toBe(stale)
  })

  it('返ってきたトークンも期限切れなら失敗にし、ストレージを書き換えない', async () => {
    const stale = staleToken()
    window.localStorage.setItem('token', stale)
    global.fetch = jest.fn(async () => ({
      ok: true,
      json: async () => ({ token: `${Math.floor(Date.now() / 1000) - 9 * 24 * 60 * 60}.older` }),
    })) as unknown as typeof fetch

    await expect(authService.ensureFreshAdminToken()).rejects.toThrow(/再ログイン/)
    expect(authService.getStoredToken()).toBe(stale)
  })

  it('失敗したあとの再呼び出しは、もう一度セッションAPIを呼ぶ', async () => {
    window.localStorage.setItem('token', staleToken())
    const fresh = `${Math.floor(Date.now() / 1000)}.new`
    const fetchMock = jest
      .fn()
      .mockResolvedValueOnce({ ok: false, status: 500, json: async () => ({}) })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ token: fresh }) })
    global.fetch = fetchMock as unknown as typeof fetch

    await expect(authService.ensureFreshAdminToken()).rejects.toThrow()
    await authService.ensureFreshAdminToken()

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(authService.getStoredToken()).toBe(fresh)
  })

  it('同時に呼ばれても、セッションAPIは1回だけ呼ぶ', async () => {
    window.localStorage.setItem('token', staleToken())
    const fresh = `${Math.floor(Date.now() / 1000)}.new`
    const fetchMock = jest.fn(async () => ({ ok: true, json: async () => ({ token: fresh }) }))
    global.fetch = fetchMock as unknown as typeof fetch

    await Promise.all([authService.ensureFreshAdminToken(), authService.ensureFreshAdminToken()])

    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})
