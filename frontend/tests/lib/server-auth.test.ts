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
  beforeEach(() => {
    jest.resetModules()
    mockCookies.mockReset()
    mockHeaders.mockReset()
    mockRedirect.mockReset()
    global.fetch = jest.fn()
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

  it('requireSessionUser は未ログイン時に /login へリダイレクトする', async () => {
    mockCookies.mockResolvedValue({ get: () => undefined })

    const { requireSessionUser } = await import('@/lib/server-auth')
    await expect(requireSessionUser()).rejects.toThrow('REDIRECT:/login')
  })

  it('requireAdminUser は非管理者を / へリダイレクトする', async () => {
    mockCookies.mockResolvedValue({
      get: (name: string) => {
        if (name === 'user_id') return { value: '1' }
        if (name === 'user_token') return { value: 'jwt' }
        return undefined
      },
    })
    mockHeaders.mockResolvedValue({ get: () => 'localhost:3000' })
    ;(global.fetch as jest.Mock).mockResolvedValue(
      new Response(
        JSON.stringify({
          user_id: 1,
          email: 'a@example.com',
          name: '学生',
          is_guest: false,
          is_admin: false,
        }),
        { status: 200 },
      ),
    )

    const { requireAdminUser } = await import('@/lib/server-auth')
    await expect(requireAdminUser()).rejects.toThrow('REDIRECT:/')
  })
})
