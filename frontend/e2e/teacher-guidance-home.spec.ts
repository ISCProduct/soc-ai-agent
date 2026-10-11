import { test, expect, type Page } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

async function silenceOtherBanners(page: Page) {
  await page.route('**/api/whats-new', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }),
  )
  await page.route('**/api/resume/status', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ has_document: true, latest_score: 90, needs_attention: false }),
    }),
  )
}

const loadFailedText = '先生からの案内を読み込めませんでした'

// #1691: 取得失敗（Error）と案内なし（Empty）を同じ見た目にしない。
test.describe('ホームの教員案内', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await silenceOtherBanners(page)
  })

  test('取得に失敗したらエラーと再読み込みを出し、再読み込みで案内が出る', async ({ page }) => {
    let fail = true
    await page.route('**/api/user/guidances', (route) =>
      fail
        ? route.fulfill({ status: 500, contentType: 'application/json', body: '{}' })
        : route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
              guidances: [{ id: 1, kind: 'low_match', message: '応募先を一緒に見直しましょう', created_at: '2026-10-01T00:00:00Z' }],
            }),
          }),
    )

    await page.goto('/')
    const errorAlert = page.getByRole('alert').filter({ hasText: loadFailedText })
    await expect(errorAlert).toBeVisible({ timeout: 15000 })

    fail = false
    await errorAlert.getByRole('button', { name: '再読み込み' }).click()
    await expect(errorAlert).toHaveCount(0)
    await expect(page.getByRole('alert').filter({ hasText: '応募先を一緒に見直しましょう' })).toBeVisible()
  })

  test('案内が0件ならエラーは出ない', async ({ page }) => {
    await page.route('**/api/user/guidances', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ guidances: [] }) }),
    )

    await page.goto('/')
    await expect(page.locator('main').first()).toBeVisible({ timeout: 15000 })
    await expect(page.getByRole('alert').filter({ hasText: loadFailedText })).toHaveCount(0)
  })
})
