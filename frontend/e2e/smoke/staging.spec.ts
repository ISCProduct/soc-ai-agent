import { test, expect } from '@playwright/test'

// 反映後スモーク（#809 プラン1）。実バックエンドに対して実行するため、
// データ作成・削除を伴う操作は行わない（Out of scope: #808）。
const apiBaseURL = process.env.PLAYWRIGHT_API_BASE_URL
if (!apiBaseURL) {
  throw new Error('PLAYWRIGHT_API_BASE_URL is required for smoke tests')
}

test.describe('反映後スモーク', () => {
  test('トップページに到達できる', async ({ page }) => {
    const response = await page.goto('/')
    expect(response?.ok()).toBeTruthy()
  })

  test('ログイン画面が表示される', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByRole('tab', { name: 'ログイン' })).toBeVisible()
    await expect(page.locator('input[type="email"]')).toBeVisible()
    await expect(page.locator('input[type="password"]')).toBeVisible()
  })

  test('バックエンドAPIのヘルスチェックが疎通する', async ({ request }) => {
    const response = await request.get(`${apiBaseURL}/healthz`)
    expect(response.ok()).toBeTruthy()
  })

  test('フロントエンドのヘルスチェックが疎通する', async ({ request, baseURL }) => {
    const response = await request.get(`${baseURL}/api/healthz`)
    expect(response.ok()).toBeTruthy()
  })
})
