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

    const { getSessionCredentials } = await import('@/lib/server-auth')
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

    const { getSessionUser } = await import('@/lib/server-auth')
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

    const { getSessionUser } = await import('@/lib/server-auth')
    const user = await getSessionUser()
    expect(user).toMatchObject({ user_id: 99, is_admin: true } satisfies Partial<User>)
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('requireSessionUser は未ログイン時に /login へリダイレクトする', async () => {
    mockCookies.mockResolvedValue({ get: () => undefined })

    const { requireSessionUser } = await import('@/lib/server-auth')
    await expect(requireSessionUser()).rejects.toThrow('REDIRECT:/login')
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

    const { requireAdminUser } = await import('@/lib/server-auth')
    await expect(requireAdminUser()).rejects.toThrow('REDIRECT:/')
  })
})
