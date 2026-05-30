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

export const TEST_USER: MockUser = {
  user_id: 1,
  email: 'test@example.com',
  name: 'テストユーザー',
  is_guest: false,
  is_admin: false,
  token: 'test-token-abc123',
  user_token: 'test-user-token-xyz789',
}

export const TEST_ADMIN: MockUser = {
  user_id: 99,
  email: 'admin@example.com',
  name: '管理者',
  is_guest: false,
  is_admin: true,
  token: 'admin-token-abc123',
  user_token: 'admin-user-token-xyz789',
}

export async function setupAuth(page: Page, user: MockUser = TEST_USER) {
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
