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
  beforeEach(() => {
    Object.defineProperty(window, 'localStorage', { value: storageMock(), configurable: true })
    Object.defineProperty(window, 'sessionStorage', { value: storageMock(), configurable: true })
  })

  afterEach(() => {
    jest.restoreAllMocks()
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
})
