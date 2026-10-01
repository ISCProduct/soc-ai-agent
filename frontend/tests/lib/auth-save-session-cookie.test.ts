/** @jest-environment jsdom */

import { authService } from '@/lib/auth'
import type { AuthResponse } from '@/lib/auth'

/**
 * saveAuth が httpOnly Cookie の設定を待ってから返すことを固定する。
 *
 * 以前は POST /api/auth/session を投げっぱなしにしていたため、呼び出し側が
 * 直後に router.push すると遷移でリクエストが破棄され、Cookie が設定されない
 * ままになっていた。ログインには成功しているのに、サーバーコンポーネントの
 * requireSessionUser() は Cookie しか見ないので保護された画面へ入れない。
 */

const AUTH_RESPONSE = {
  user_id: 42,
  email: 'student@example.com',
  name: '山田太郎',
  is_guest: false,
  is_admin: false,
  token: 'admin-token',
  user_token: 'user-token',
  refresh_token: 'refresh-token',
} as unknown as AuthResponse

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

describe('authService.saveAuth', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'localStorage', { value: storageMock(), configurable: true })
    Object.defineProperty(window, 'sessionStorage', { value: storageMock(), configurable: true })
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('Cookie 設定の完了を待ってから返す', async () => {
    let settled = false
    const fetchMock = jest.fn(
      () =>
        new Promise<Response>((resolve) => {
          setTimeout(() => {
            settled = true
            resolve({ ok: true, status: 200, json: async () => ({}) } as Response)
          }, 20)
        }),
    )
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.saveAuth(AUTH_RESPONSE)

    // 投げっぱなしだとここが false のまま返ってくる。
    expect(settled).toBe(true)
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/session',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('送る内容は userId / userToken / refreshToken', async () => {
    // mock.calls から引数を読むので、fetch と同じ引数型で宣言する。
    const fetchMock = jest.fn<Promise<Response>, [RequestInfo | URL, RequestInit?]>(
      async () => ({ ok: true, status: 200 }) as Response,
    )
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.saveAuth(AUTH_RESPONSE)

    const init = fetchMock.mock.calls[0][1]
    const body = JSON.parse(init?.body as string)
    expect(body).toEqual({
      userId: 42,
      userToken: 'user-token',
      refreshToken: 'refresh-token',
    })
  })

  it('Cookie 設定に失敗しても投げない（localStorage は残す）', async () => {
    // 一時障害で例外にすると、呼び出し側がログイン全体を失敗扱いにしてしまう。
    global.fetch = jest.fn(async () => {
      throw new Error('network down')
    }) as unknown as typeof fetch

    await expect(authService.saveAuth(AUTH_RESPONSE)).resolves.toBeUndefined()
    expect(authService.getStoredUser()?.email).toBe('student@example.com')
  })

  it('user_token が無いときは Cookie を設定しにいかない', async () => {
    const fetchMock = jest.fn(async () => ({ ok: true }) as Response)
    global.fetch = fetchMock as unknown as typeof fetch

    await authService.saveAuth({ ...AUTH_RESPONSE, user_token: undefined } as AuthResponse)

    expect(fetchMock).not.toHaveBeenCalled()
  })
})
