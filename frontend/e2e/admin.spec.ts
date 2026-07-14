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
    await expect(page.getByRole('heading', { name: 'AI / RAG 運用' })).toBeVisible({ timeout: 8000 })
    await expect(page.getByRole('heading', { name: 'スコアダッシュボード' })).toBeVisible({ timeout: 8000 })
    await expect(page.getByRole('heading', { name: 'スコア精度検証' })).toBeVisible({ timeout: 8000 })
  })

  test('AI / RAG 運用ページに遷移できる', async ({ page }) => {
    await page.route('**/api/admin/companies/l1-coverage', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          published_total: 10,
          info_fresh: 8,
          has_profile: 7,
          needs_warm: 2,
          info_rate: 0.8,
          profile_rate: 0.7,
        }),
      })
    })
    await page.route('**/api/admin/vector/status**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          backend: 'chroma',
          total_documents: 42,
          collections: [{ name: 'company_research', exists: true, count: 42 }],
        }),
      })
    })
    await page.route('**/api/admin/costs**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          current_month_cost_usd: 12.5,
          company_search: { month: '2026-07', count: 100, limit: 2000, remaining: 1900, enforce: true, exceeded: false },
          realtime: { current_month_cost_usd: 1.2 },
        }),
      })
    })

    await page.goto('/admin/ai-ops')
    await page.waitForLoadState('networkidle')
    await expect(page.getByRole('heading', { name: 'AI / RAG 運用' })).toBeVisible({ timeout: 8000 })
    await expect(page.getByText('コスト削減の基本方針')).toBeVisible({ timeout: 8000 })
    await expect(page.getByRole('button', { name: 'ウォーム（キャッシュ優先・安価）' })).toBeVisible()
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
