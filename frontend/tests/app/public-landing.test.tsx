/** @jest-environment jsdom */

/**
 * 公開LP（#1653）の退行テスト。
 *
 * ここで押さえるのは「気づかないうちに壊れる」3点。
 *
 * 1. ログイン済みに LP が出る — 認証分岐を逆に書いても画面は表示されるので気づけない
 * 2. Backend 停止中に LP が落ちる — ローカルでは Backend が動いているので再現しない。
 *    本番は展示会運用で desired=0 から起動するため、ここが壊れると来場者に
 *    エラー画面が出る
 *
 * 3点目（未ログインに学生用ナビが出る）は components/student-bottom-nav-auth.test.tsx。
 */

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

/** Cookie が無い＝未ログイン。 */
function noCookies() {
  mockCookies.mockResolvedValue({ get: () => undefined })
}

/** Cookie あり＝セッションがある状態。 */
function withCookies() {
  mockCookies.mockResolvedValue({
    get: (name: string) => {
      if (name === 'user_id') return { value: '42' }
      if (name === 'user_token') return { value: 'token-abc' }
      return undefined
    },
  })
}

describe('公開LP', () => {
  const originalE2eMockAuth = process.env.E2E_MOCK_AUTH

  beforeEach(() => {
    jest.resetModules()
    mockCookies.mockReset()
    mockHeaders.mockReset()
    mockRedirect.mockReset()
    mockHeaders.mockResolvedValue({ get: () => '' })
    global.fetch = jest.fn()
    delete process.env.E2E_MOCK_AUTH
  })

  afterAll(() => {
    process.env.E2E_MOCK_AUTH = originalE2eMockAuth
  })

  it('未ログインなら Backend を一切呼ばずに LP を返す', async () => {
    noCookies()

    const Page = (await import('@/app/page')).default
    const element = await Page()

    // Backend 停止中（本番の非稼働日・起動途中）でも表示できる必要がある。
    // Cookie が無い時点で getSessionCredentials が null を返し、
    // getSessionUser は fetch へ到達しない。
    expect(global.fetch).not.toHaveBeenCalled()

    const { LandingContent } = await import('@/components/landing/LandingContent')
    expect(element.type).toBe(LandingContent)
  })

  it('未ログインでも /login へリダイレクトしない', async () => {
    noCookies()

    const Page = (await import('@/app/page')).default
    await Page()

    // 以前は requireSessionUser() が redirect('/login') していた。
    expect(mockRedirect).not.toHaveBeenCalled()
  })

  it('ログイン済みなら LP ではなく従来のダッシュボードを返す', async () => {
    withCookies()
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({ id: 42, email: 'student@example.com', name: '山田太郎' }),
    })

    const Page = (await import('@/app/page')).default
    const element = await Page()

    const { LandingContent } = await import('@/components/landing/LandingContent')
    const PageContent = (await import('@/app/page-content')).default

    expect(element.type).not.toBe(LandingContent)
    expect(element.type).toBe(PageContent)
  })

  it('LP は認証APIを含むデータ取得をしない（Server Component のまま保つ）', async () => {
    const source = await import('fs').then((fs) =>
      fs.readFileSync('components/landing/LandingContent.tsx', 'utf8'),
    )

    // 'use client' を付けると未ログインの訪問者へ不要なJSを配る。
    expect(source).not.toMatch(/^['"]use client['"]/m)
    // fetch を足すと Backend 停止中に落ちる。
    expect(source).not.toMatch(/\bfetch\s*\(/)
  })
})
