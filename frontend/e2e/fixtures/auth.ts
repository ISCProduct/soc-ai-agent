import { Page } from '@playwright/test'

export type MockUser = {
  user_id: number
  email: string
  name: string
  is_guest: boolean
  is_admin?: boolean
  token: string
  user_token: string
}

/**
 * テスト用の JWT を組み立てる。
 *
 * Backend の GenerateJWT は必ず exp 付きで発行する。ここを不透明な文字列にすると
 * middleware が「読めない＝期限切れ」と判定して Cookie を落とすため(#1535)、
 * ログイン済みのはずの全テストが未ログイン扱いになる。署名は検証しないので
 * 中身だけ実物と揃える。
 */
export function testJwt(userId: number, expiresInSeconds = 60 * 60): string {
  const b64 = (o: unknown) => Buffer.from(JSON.stringify(o)).toString('base64url')
  const exp = Math.floor(Date.now() / 1000) + expiresInSeconds
  return `${b64({ alg: 'HS256', typ: 'JWT' })}.${b64({ sub: String(userId), exp })}.test-signature`
}

export const TEST_USER: MockUser = {
  user_id: 1,
  email: 'test@example.com',
  name: 'テストユーザー',
  is_guest: false,
  is_admin: false,
  token: 'test-token-abc123',
  user_token: testJwt(1),
}

export const TEST_ADMIN: MockUser = {
  user_id: 99,
  email: 'admin@example.com',
  name: '管理者',
  is_guest: false,
  is_admin: true,
  token: 'admin-token-abc123',
  user_token: testJwt(99),
}

export async function setupAuth(page: Page, user: MockUser = TEST_USER) {
  const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000'

  await page.context().addCookies([
    {
      name: 'user_id',
      value: String(user.user_id),
      url: baseURL,
      httpOnly: true,
      sameSite: 'Lax',
    },
    {
      name: 'user_token',
      value: user.user_token,
      url: baseURL,
      httpOnly: true,
      sameSite: 'Lax',
    },
  ])

  await page.addInitScript(
    ({ u }: { u: MockUser }) => {
      const userData = {
        user_id: u.user_id,
        email: u.email,
        name: u.name,
        is_guest: u.is_guest,
        is_admin: u.is_admin,
      }
      sessionStorage.setItem('user', JSON.stringify(userData))
      sessionStorage.setItem('token', u.token)
      sessionStorage.setItem('user_token', u.user_token)
      localStorage.setItem('chat_session_id', 'test-session-id-001')
    },
    { u: user },
  )
}
