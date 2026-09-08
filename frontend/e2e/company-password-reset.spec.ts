import { test, expect } from '@playwright/test'

const RESET_SENT_MESSAGE = /メールを送信しました/

test.describe('企業ポータル パスワードリセット', () => {
  test('リセットメール送信後に完了メッセージが表示される', async ({ page }) => {
    await page.route('**/api/company-auth/forgot-password', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'パスワード再設定用のメールを送信しました' }),
      })
    })

    await page.goto('/company-portal/forgot-password')
    await page.waitForLoadState('networkidle')
    await page.locator('input[type="email"]').fill('hr@example.com')
    await page.getByRole('button', { name: 'リセットメールを送る' }).click()

    await expect(page.getByText(RESET_SENT_MESSAGE)).toBeVisible({ timeout: 5000 })
  })

  test('存在しないメールアドレスでも同じ完了メッセージが表示される', async ({ page }) => {
    // Backendは存在有無に関わらず200を返す。フロントも同じ表示にすることを検証する (#1196)
    await page.route('**/api/company-auth/forgot-password', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'パスワード再設定用のメールを送信しました' }),
      })
    })

    await page.goto('/company-portal/forgot-password')
    await page.waitForLoadState('networkidle')
    await page.locator('input[type="email"]').fill('not-registered@example.com')
    await page.getByRole('button', { name: 'リセットメールを送る' }).click()

    await expect(page.getByText(RESET_SENT_MESSAGE)).toBeVisible({ timeout: 5000 })
    // 「登録されていません」等、アカウントの存在有無を示す表示が無いこと
    await expect(page.getByText(/登録されていません|見つかりません/)).toHaveCount(0)
  })

  test('パスワード再設定に成功すると企業ポータルへ遷移する', async ({ page }) => {
    const authResponse = {
      company_user_id: 1,
      company_id: 10,
      email: 'hr@example.com',
      name: '採用担当',
      role: 'member',
      token: 'mock-company-token',
      refresh_token: 'mock-company-refresh-token',
    }

    await page.route('**/api/company-auth/reset-password', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(authResponse),
      })
    })
    await page.route('**/api/company-auth/session', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) })
    })
    await page.route('**/api/company-auth/me', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(authResponse) })
    })

    await page.goto('/company-portal/reset-password?token=valid-token')
    await page.waitForLoadState('networkidle')
    await page.locator('input[type="password"]').first().fill('newpassword123')
    await page.locator('input[type="password"]').last().fill('newpassword123')
    await page.getByRole('button', { name: 'パスワードを再設定する' }).click()

    await expect(page).toHaveURL(/\/company-portal$/, { timeout: 10000 })
  })

  test('パスワードが一致しない場合はエラーを表示して送信しない', async ({ page }) => {
    let called = false
    await page.route('**/api/company-auth/reset-password', async (route) => {
      called = true
      await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
    })

    await page.goto('/company-portal/reset-password?token=valid-token')
    await page.waitForLoadState('networkidle')
    await page.locator('input[type="password"]').first().fill('newpassword123')
    await page.locator('input[type="password"]').last().fill('different123')
    await page.getByRole('button', { name: 'パスワードを再設定する' }).click()

    await expect(page.getByRole('alert').filter({ hasText: 'パスワードが一致しません。' })).toBeVisible()
    expect(called).toBe(false)
  })

  test('ログイン画面からパスワードリセット画面へ遷移できる', async ({ page }) => {
    await page.goto('/company-portal/sign-in')
    await page.waitForLoadState('networkidle')
    await page.getByRole('link', { name: 'パスワードをお忘れですか？' }).click()

    await expect(page).toHaveURL(/\/company-portal\/forgot-password$/)
  })
})
