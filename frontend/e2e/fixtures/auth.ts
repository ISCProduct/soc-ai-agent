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
 * テスト用の user_token を作る（#1535）。
 *
 * middleware は署名を検証せず exp だけ見てリフレッシュ契機を決める。
 * exp が読めないトークンは**期限切れとして扱われ**、後段へ注入されない。
 *
 * 本番の Backend（`GenerateJWT`）は必ず exp 付きの JWT を発行するので、
 * ダミー文字列を置くと実態と違う条件でテストすることになる。
 * 以前は 'test-user-token-xyz789' のような文字列で、middleware が
 * フェイルオープンだったからたまたま通っていた。
 */
function testJwt(sub: number, expiresInSeconds = 60 * 60 * 24): string {
  const b64 = (o: unknown) => Buffer.from(JSON.stringify(o)).toString('base64url')
  const exp = Math.floor(Date.now() / 1000) + expiresInSeconds
  return `${b64({ alg: 'HS256', typ: 'JWT' })}.${b64({ sub: String(sub), exp })}.e2e-signature`
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
