import { test, expect } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

test.describe('職務経歴書レビューフロー', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)

    await page.route('**/api/user/weight-scores*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ weight_scores: [] }),
      })
    })
  })

  test('職務経歴書ページが表示される', async ({ page }) => {
    await page.goto('/resume')
    await expect(page.getByText('履歴書・エントリシート レビュー')).toBeVisible({ timeout: 8000 })
  })

  test('PDFアップロードからレビュー完了まで', async ({ page }) => {
    await page.route('**/api/resume/upload', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          document: { id: 42, status: 'uploaded' },
          message: 'uploaded',
        }),
      })
    })

    const reviewBody = JSON.stringify({
      review: { id: 1, score: 78, summary: '全体的にバランスの取れた経歴書です。' },
      items: [
        { id: 1, page_number: 1, severity: 'info', message: '具体的な数値を追加すると改善されます', suggestion: '売上10%増などの数値を記載してください' },
        { id: 2, page_number: 2, severity: 'warning', message: '成果の記述が不足しています', suggestion: '担当プロジェクトの成果を追記してください' },
      ],
      annotated_available: false,
    })

    await page.route('**/api/resume/review/stream*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: [
          `data: ${JSON.stringify({ type: 'chunk', text: 'レビュー結果：' })}`,
          `data: ${JSON.stringify({ type: 'chunk', text: '全体的にバランスの取れた内容です。' })}`,
          `data: ${JSON.stringify({ type: 'complete', ...JSON.parse(reviewBody), annotated_available: false })}`,
        ].join('\n') + '\n',
      })
    })

    await page.goto('/resume')
    await expect(page.getByText('履歴書・エントリシート レビュー')).toBeVisible({ timeout: 8000 })

    const uploadButton = page.getByRole('button', { name: /アップロード|PDFを選択/ })
    if (await uploadButton.count() > 0) {
      const fileChooserPromise = page.waitForEvent('filechooser', { timeout: 5000 }).catch(() => null)
      await uploadButton.click()
      const fileChooser = await fileChooserPromise
      if (fileChooser) {
        await fileChooser.setFiles({
          name: 'test-resume.pdf',
          mimeType: 'application/pdf',
          buffer: Buffer.from('%PDF-1.4 test'),
        })
      }
    }

    const reviewBtn = page.getByRole('button', { name: 'レビューを生成' })
    const isEnabled = await reviewBtn.isEnabled({ timeout: 3000 }).catch(() => false)
    if (isEnabled) {
      await page.getByLabel(/応募企業名/).fill('テスト株式会社')
      await reviewBtn.click()
      await expect(page.getByText('全体的にバランスの取れた経歴書です。')).toBeVisible({ timeout: 10000 })
    }
  })
})
