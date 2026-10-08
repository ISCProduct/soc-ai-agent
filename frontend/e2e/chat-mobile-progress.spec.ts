import { test, expect, type Page } from '@playwright/test'

/** モバイル幅でチャットの現在地がどこまで見えるかを確認する。 */
const EMAIL = process.env.E2E_EMAIL ?? 'uicheck.local@example.com'
const PASSWORD = process.env.E2E_PASSWORD ?? ''

async function login(page: Page) {
  await page.goto('/login')
  await page.getByRole('textbox', { name: 'メールアドレス' }).fill(EMAIL)
  await page.locator('input[type="password"]').fill(PASSWORD)
  await page.getByRole('button', { name: 'ログイン', exact: true }).click()
  await page.waitForURL('**/', { timeout: 30000 })
}

test.describe('モバイルの現在地表示', () => {
  test.skip(!PASSWORD, 'E2E_PASSWORD が未設定')
  test.use({ viewport: { width: 390, height: 844 } })

  test('進捗はモバイルでも見える（項目別の内訳はドロワー内）', async ({ page }) => {
    // 「モバイルでは現在地が全く見えない」と判断しかけたが、実機では
    // 進捗テキストと棒は出ていた。隠れているのは項目別のチェックリストだけ。
    // 取り違えると不要な改修に向かうので、両方を固定する。
    await login(page)
    await page.waitForTimeout(4000)
    await page.screenshot({ path: 'test-results/chat-mobile.png' })

    const body = await page.locator('body').innerText()
    expect(body).toMatch(/問に回答/)
    expect(body).not.toContain('ここまで聞いたこと')
  })
})
