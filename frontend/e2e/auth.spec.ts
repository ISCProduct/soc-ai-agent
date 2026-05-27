import { test, expect } from '@playwright/test'

test.describe('認証フロー', () => {
  test('ログインページが表示される', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByRole('tab', { name: 'ログイン' })).toBeVisible()
    await expect(page.locator('input[type="email"]')).toBeVisible()
    await expect(page.locator('input[type="password"]')).toBeVisible()
  })

  test('メールアドレスとパスワードでログインできる', async ({ page }) => {
    await page.route('**/api/auth/login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          user_id: 1,
          email: 'test@example.com',
          name: 'テストユーザー',
          token: 'mock-token',
          user_token: 'mock-user-token',
          is_guest: false,
        }),
      })
    })

    await page.goto('/login')
    await page.locator('input[type="email"]').fill('test@example.com')
    await page.locator('input[type="password"]').fill('password123')
    await page.getByRole('button', { name: 'ログイン', exact: true }).click()

    await expect(page).not.toHaveURL(/\/login/, { timeout: 10000 })
  })

  test('無効な認証情報でエラーメッセージが表示される', async ({ page }) => {
    await page.route('**/api/auth/login', async (route) => {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'UNAUTHORIZED', error: 'invalid email or password' }),
      })
    })

    await page.goto('/login')
    await page.locator('input[type="email"]').fill('wrong@example.com')
    await page.locator('input[type="password"]').fill('wrongpassword')
    await page.getByRole('button', { name: 'ログイン', exact: true }).click()

    const errorAlert = page.getByRole('alert').filter({ hasText: /invalid email or password/ })
    await expect(errorAlert).toBeVisible({ timeout: 5000 })
  })

  test('仮登録メールアドレス送信', async ({ page }) => {
    await page.route('**/api/auth/request-registration', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'confirmation email sent' }),
      })
    })

    await page.goto('/login')
    await page.getByRole('tab', { name: '新規登録' }).click()
    await page.locator('input[type="email"]').last().fill('newuser@example.com')
    await page.getByRole('button', { name: '確認メールを送る' }).click()

    await expect(page.getByText(/確認リンクを送りました/)).toBeVisible({ timeout: 5000 })
  })
})
