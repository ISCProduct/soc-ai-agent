import type { User } from '@/lib/auth'

const mockCookies = jest.fn()
const mockHeaders = jest.fn()
const mockRedirect = jest.fn()

jest.mock('next/headers', () => ({
  cookies: () => mockCookies(),
  headers: () => mockHeaders(),
}))

jest.mock('next/navigation', () => ({
  redirect: (path: string) => {
    mockRedirect(path)
    throw new Error(`REDIRECT:${path}`)
  },
}))

describe('server-auth', () => {
  const originalE2eMockAuth = process.env.E2E_MOCK_AUTH

  beforeEach(() => {
    jest.resetModules()
    mockCookies.mockReset()
    mockHeaders.mockReset()
    mockRedirect.mockReset()
    global.fetch = jest.fn()
    process.env.E2E_MOCK_AUTH = originalE2eMockAuth
  })

  afterAll(() => {
    process.env.E2E_MOCK_AUTH = originalE2eMockAuth
  })

  it('getSessionCredentials は cookie から資格情報を返す', async () => {
    mockCookies.mockResolvedValue({
      get: (name: string) => {
        if (name === 'user_id') return { value: '42' }
        if (name === 'user_token') return { value: 'token-abc' }
        return undefined
      },
    })

    const { getSessionCredentials } = await import('@/lib/auth/server')
    await expect(getSessionCredentials()).resolves.toEqual({
      userId: '42',
      userToken: 'token-abc',
    })
  })

  it('getSessionUser は Backend からユーザーを取得する', async () => {
    process.env.E2E_MOCK_AUTH = 'false'
    mockCookies.mockResolvedValue({
      get: (name: string) => {
        if (name === 'user_id') return { value: '1' }
        if (name === 'user_token') return { value: 'jwt' }
        return undefined
      },
    })
    mockHeaders.mockResolvedValue({
      get: () => 'localhost:3000',
    })
    ;(global.fetch as jest.Mock).mockResolvedValue(
      new Response(
        JSON.stringify({
          user_id: 1,
          email: 'a@example.com',
          name: 'テスト',
          is_guest: false,
        }),
        { status: 200 },
      ),
    )

    const { getSessionUser } = await import('@/lib/auth/server')
    const user = await getSessionUser()
    expect(user).toMatchObject({ user_id: 1, email: 'a@example.com' } satisfies Partial<User>)
  })

  it('E2E_MOCK_AUTH 時は Backend なしでユーザーを返す', async () => {
    process.env.E2E_MOCK_AUTH = 'true'
    mockCookies.mockResolvedValue({
      get: (name: string) => {
        if (name === 'user_id') return { value: '99' }
        if (name === 'user_token') return { value: 'jwt' }
        return undefined
      },
    })

    const { getSessionUser } = await import('@/lib/auth/server')
    const user = await getSessionUser()
    expect(user).toMatchObject({ user_id: 99, is_admin: true } satisfies Partial<User>)
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('requireSessionUser は未ログイン時に /login へリダイレクトする', async () => {
    mockCookies.mockResolvedValue({ get: () => undefined })

    const { requireSessionUser } = await import('@/lib/auth/server')
    await expect(requireSessionUser()).rejects.toThrow('REDIRECT:/login')
  })

  // Backend 停止時に fetch が投げる例外が Server Component を突き抜けると、
  // requireSessionUser() を呼ぶ全ページが 500 になる（TypeError: fetch failed）。
  const loggedInCookies = {
    get: (name: string) => {
      if (name === 'user_id') return { value: '1' }
      if (name === 'user_token') return { value: 'jwt' }
      return undefined
    },
  }

  it('getSessionUser は Backend が落ちていても例外を投げず null を返す', async () => {
    process.env.E2E_MOCK_AUTH = 'false'
    mockCookies.mockResolvedValue(loggedInCookies)
    mockHeaders.mockResolvedValue({ get: () => 'localhost:3000' })
    ;(global.fetch as jest.Mock).mockRejectedValue(
      Object.assign(new TypeError('fetch failed'), {
        cause: new Error('connect ECONNREFUSED 172.24.0.4:8080'),
      }),
    )

    const { getSessionUser } = await import('@/lib/auth/server')
    await expect(getSessionUser()).resolves.toBeNull()
  })

  it('requireSessionUser は Backend が落ちていると /login へ倒す（500にしない）', async () => {
    process.env.E2E_MOCK_AUTH = 'false'
    mockCookies.mockResolvedValue(loggedInCookies)
    mockHeaders.mockResolvedValue({ get: () => 'localhost:3000' })
    ;(global.fetch as jest.Mock).mockRejectedValue(new TypeError('fetch failed'))

    const { requireSessionUser } = await import('@/lib/auth/server')
    await expect(requireSessionUser()).rejects.toThrow('REDIRECT:/login')
  })

  // 本文の読み込みで失敗する場合（応答が途中で切れる等）も同じく倒す。
  it('getSessionUser は本文の読み込みに失敗しても null を返す', async () => {
    process.env.E2E_MOCK_AUTH = 'false'
    mockCookies.mockResolvedValue(loggedInCookies)
    mockHeaders.mockResolvedValue({ get: () => 'localhost:3000' })
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.reject(new Error('unexpected end of JSON input')),
    })

    const { getSessionUser } = await import('@/lib/auth/server')
    await expect(getSessionUser()).resolves.toBeNull()
  })

  it('requireAdminUser は非管理者を / へリダイレクトする', async () => {
    process.env.E2E_MOCK_AUTH = 'true'
    mockCookies.mockResolvedValue({
      get: (name: string) => {
        if (name === 'user_id') return { value: '1' }
        if (name === 'user_token') return { value: 'jwt' }
        return undefined
      },
    })

    const { requireAdminUser } = await import('@/lib/auth/server')
    await expect(requireAdminUser()).rejects.toThrow('REDIRECT:/')
  })
})
