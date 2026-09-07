import { test, expect, type Page } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'
import type { ResumeStatus } from '../lib/resume-reminder'

async function mockResumeStatus(page: Page, status: ResumeStatus) {
  await page.route('**/api/resume/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(status),
    })
  })
}

// 同じ画面の「新着情報」バナーも role=alert かつ閉じるボタンを持つため、
// 空にしておかないとセレクタが2件に解決して strict mode 違反になる。
async function silenceWhatsNew(page: Page) {
  await page.route('**/api/whats-new', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
  })
}

/** 履歴書リマインダーの Alert だけに絞り込む。 */
function reminderAlert(page: Page) {
  return page.getByRole('alert').filter({ hasText: /履歴書/ })
}

test.describe('履歴書リマインダーカード', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await silenceWhatsNew(page)
  })

  test('未提出のときリマインダーが表示される', async ({ page }) => {
    await mockResumeStatus(page, { has_document: false, latest_score: null, needs_attention: true })

    await page.goto('/')
    await expect(reminderAlert(page)).toContainText('履歴書がまだ作成されていません')
    await expect(reminderAlert(page).getByRole('link', { name: '履歴書を確認する' })).toHaveAttribute(
      'href',
      '/resume',
    )
  })

  test('アップ済みでレビュー未実施ならレビューを促す', async ({ page }) => {
    await mockResumeStatus(page, { has_document: true, latest_score: null, needs_attention: true })

    await page.goto('/')
    await expect(reminderAlert(page)).toContainText('レビューをまだ受けていません')
  })

  // MUI の Alert は action を渡すと onClose の閉じるボタンを描画しないため、
  // 導線は本文に置いている。action を戻すとこのテストが落ちる。
  test('閉じるボタンでリマインダーを消せる', async ({ page }) => {
    await mockResumeStatus(page, { has_document: true, latest_score: 45, needs_attention: true })

    await page.goto('/')
    const alert = reminderAlert(page)
    await expect(alert).toContainText('評価が低めです')

    await alert.getByRole('button').click()
    await expect(alert).toHaveCount(0)
  })

  test('スコア80のときリマインダーは表示されない', async ({ page }) => {
    await mockResumeStatus(page, { has_document: true, latest_score: 80, needs_attention: false })

    await page.goto('/')
    // 「まだ描画されていないだけ」で0件になるのを避けるため、
    // 先に画面本体の描画完了を待ってから0件を主張する。
    await expect(page.getByRole('button', { name: 'menu' }).or(page.locator('header'))).toBeVisible({
      timeout: 15000,
    })
    await expect(reminderAlert(page)).toHaveCount(0)
  })
})
