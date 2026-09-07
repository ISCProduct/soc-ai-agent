import { test, expect } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'
import type { ResumeStatus } from '../lib/resume-reminder'

async function mockResumeStatus(page: import('@playwright/test').Page, status: ResumeStatus) {
  await page.route('**/api/resume/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(status),
    })
  })
}

test.describe('履歴書リマインダーカード', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)
  })

  test('未提出のときリマインダーが表示される', async ({ page }) => {
    await mockResumeStatus(page, { has_document: false, latest_score: null, needs_attention: true })

    await page.goto('/')
    await expect(page.getByText('履歴書がまだ作成されていません')).toBeVisible({ timeout: 10000 })
    await expect(page.getByRole('link', { name: '履歴書を確認する' })).toHaveAttribute('href', '/resume')
  })

  test('スコア80のときリマインダーは表示されない', async ({ page }) => {
    await mockResumeStatus(page, { has_document: true, latest_score: 80, needs_attention: false })

    const statusResponse = page.waitForResponse('**/api/resume/status')
    await page.goto('/')
    await statusResponse
    await expect(page.getByText(/履歴書がまだ作成されていません|評価が低めです/)).toHaveCount(0)
  })
})
