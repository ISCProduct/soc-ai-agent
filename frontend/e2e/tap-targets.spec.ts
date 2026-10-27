import { test, expect, type Locator, type Page } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

/**
 * #1481: モバイルのタップ領域は高さ44px以上。
 * テキストリンクは幅が広がってよい。アイコンボタンは幅も44px以上。
 */
test.use({ viewport: { width: 390, height: 844 }, hasTouch: true })

async function expectMinBox(locator: Locator, minHeight: number, minWidth?: number) {
  await expect(locator).toBeVisible()
  const box = await locator.boundingBox()
  expect(box).not.toBeNull()
  expect(box!.height).toBeGreaterThanOrEqual(minHeight)
  if (minWidth !== undefined) {
    expect(box!.width).toBeGreaterThanOrEqual(minWidth)
  }
}

test.describe('モバイルのタップ領域（#1481）', () => {
  test('ログイン導線のテキストリンクは高さ44px以上', async ({ page }) => {
    await page.context().clearCookies()

    await page.goto('/login')
    await expectMinBox(page.getByRole('link', { name: 'パスワードをお忘れですか？' }), 44)

    await page.goto('/company-portal/sign-in')
    await expectMinBox(page.getByRole('link', { name: 'パスワードをお忘れですか？' }), 44)

    await page.goto('/forgot-password')
    await expectMinBox(page.getByRole('link', { name: 'ログインページへ戻る' }), 44)

    await page.goto('/reset-password')
    await expectMinBox(page.getByRole('link', { name: 'ログインページへ戻る' }), 44)

    await page.goto('/company-portal/forgot-password')
    await expectMinBox(page.getByRole('link', { name: 'ログインページへ戻る' }), 44)
  })

  test('企業登録のプライバシーポリシーは高さ44px以上', async ({ page }) => {
    await page.context().clearCookies()
    await page.goto('/company-entry')
    await expectMinBox(page.getByRole('link', { name: 'プライバシーポリシー' }), 44)
  })

  test('履歴書の戻るボタンは44x44以上', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await page.goto('/resume')
    await expectMinBox(page.getByRole('button', { name: '前のページに戻る' }), 44, 44)
  })

  test('面接履歴の期間フィルタは高さ44px以上', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await mockInterviewHistory(page)
    await page.goto('/interview/history')
    for (const name of ['直近5回', '直近10回', '全期間']) {
      await expectMinBox(page.getByRole('button', { name }), 44)
    }
  })
})

async function mockInterviewHistory(page: Page) {
  await page.route('**/api/interviews/trend*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ points: [] }),
    }),
  )
  await page.route('**/api/interviews?*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ sessions: [], total: 0 }),
    }),
  )
}
