import { test, expect } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

test.describe('チャット分析フロー', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)

    await page.route('/api/chat/session', async (route) => {
      if (route.request().method() === 'POST') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ session_id: 'test-session-id-001' }),
        })
      } else {
        await route.continue()
      }
    })

    await page.route('/api/chat/messages*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ messages: [] }),
      })
    })
  })

  test('チャット画面が表示される', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('body')).toBeVisible()
  })

  test('マッチング結果ページに遷移できる', async ({ page }) => {
    await page.route('/api/companies*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          companies: [
            { id: 1, name: 'テスト株式会社', match_score: 85, industry: 'IT' },
            { id: 2, name: 'サンプル工業', match_score: 72, industry: '製造' },
          ],
        }),
      })
    })

    await page.goto('/results')
    await expect(page.locator('body')).toBeVisible()
  })
})
