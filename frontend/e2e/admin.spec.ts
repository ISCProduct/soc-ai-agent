import { test, expect } from '@playwright/test'
import { setupAuth, TEST_ADMIN } from './fixtures/auth'

test.describe('管理者ダッシュボードフロー', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_ADMIN)

    await page.route('**/api/admin/companies*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          companies: [
            { id: 1, name: 'テスト株式会社', status: 'published' },
            { id: 2, name: 'サンプル工業', status: 'draft' },
          ],
        }),
      })
    })

    await page.route('**/api/admin/crawl-sources*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ sources: [] }),
      })
    })

    await page.route(/\/api\/admin\/dashboard\/users/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          users: [
            { user_id: 1, name: 'ユーザー1', email: 'user1@example.com', role: '新卒', registered_at: '2025-01-01T00:00:00Z', session_count: 3, last_session_at: null, avg_score: 3.5 },
          ],
          total: 1,
        }),
      })
    })
  })

  test('管理者ダッシュボードが表示される', async ({ page }) => {
    await page.goto('/admin')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText('管理者ダッシュボード')).toBeVisible({ timeout: 8000 })
  })

  test('管理者ダッシュボードにメニューカードが表示される', async ({ page }) => {
    await page.goto('/admin')
    await page.waitForLoadState('networkidle')
    await expect(page.getByRole('heading', { name: '企業データ' })).toBeVisible({ timeout: 8000 })
    await expect(page.getByRole('heading', { name: 'スコアダッシュボード' })).toBeVisible({ timeout: 8000 })
    await expect(page.getByRole('heading', { name: 'スコア精度検証' })).toBeVisible({ timeout: 8000 })
  })

  test('スコアダッシュボードページに遷移できる', async ({ page }) => {
    await page.goto('/admin/dashboard')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText('ユーザー別スコアダッシュボード')).toBeVisible({ timeout: 8000 })
    await expect(page.getByText('ユーザー1')).toBeVisible({ timeout: 8000 })
  })

  test('スコア精度検証ページが表示される', async ({ page }) => {
    await page.route('**/api/admin/score-validation/*', async (route) => {
      const url = route.request().url()
      if (url.includes('/correlation')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ generated_at: new Date().toISOString(), total_candidates: 0, categories: [] }),
        })
      } else if (url.includes('/phase-metrics')) {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ generated_at: new Date().toISOString(), phases: [] }),
        })
      } else {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({}) })
      }
    })

    await page.goto('/admin/score-validation')
    await page.waitForLoadState('networkidle')
    await expect(page.getByText('スコア精度検証')).toBeVisible({ timeout: 8000 })
    await expect(page.getByRole('tab', { name: '相関分析' })).toBeVisible()
    await expect(page.getByRole('tab', { name: 'A/Bテスト管理' })).toBeVisible()
  })
})

test.describe('非管理者アクセス制御', () => {
  test('管理者以外はリダイレクトされる', async ({ page }) => {
    await page.addInitScript(() => {
      const user = { user_id: 1, email: 'normal@example.com', name: 'Normal', is_guest: false, is_admin: false }
      sessionStorage.setItem('user', JSON.stringify(user))
      sessionStorage.setItem('token', 'normal-token')
    })

    await page.goto('/admin')
    await page.waitForURL((url) => !url.pathname.startsWith('/admin'), { timeout: 10000 })
  })
})
