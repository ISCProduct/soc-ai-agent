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
    // autocomplete が外れるとパスワードマネージャが効かず、利用者はログインできたまま
    // 「保存されない」で詰まる。画面は正常に見えるので人の目視では気づけない。
    await expect(page.locator('input[type="password"]')).toHaveAttribute(
      'autocomplete',
      'current-password',
    )
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

// ローカル統合テスト環境（compose.test.yml、SOCAIAGENT-local-e2e-infra）専用。
// 他の e2e/*.spec.ts はほぼ全て page.route() で API をモックしており
// Backend の実際の状態を検証しない。このケースだけは route モックを使わず、
// Backend/cmd/seed-e2e が作る実ユーザーで実際に /api/auth/login を通す。
// 認証情報は Backend/cmd/seed-e2e/main.go の定数と合わせている（変更時は両方）。
//
// このspecはdeploy-smoke.ymlで staging/production にも向けて実行される。
// シードした e2e-student@example.com はローカル統合テスト環境にしか存在しないため、
// E2E_LOCAL_INTEGRATION=true が無い実行では必ずskipする（本番ログインで落とさない）。
test.describe('実Backend結合（ローカル統合テスト環境専用）', () => {
  test.skip(
    process.env.E2E_LOCAL_INTEGRATION !== 'true',
    'ローカル統合テスト環境専用（make test-e2e が E2E_LOCAL_INTEGRATION=true を渡す）',
  )

  test('シードした実ユーザーで実ログインできる', async ({ page }) => {
    await page.goto('/login')
    await page.waitForLoadState('networkidle')
    await page.locator('input[type="email"]').fill('e2e-student@example.com')
    await page.locator('input[type="password"]').fill('e2e-test-password-local-only')
    await page.getByRole('button', { name: 'ログイン', exact: true }).click()

    await expect(page).not.toHaveURL(/\/login/, { timeout: 10000 })
  })
})
