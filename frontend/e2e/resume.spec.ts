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
    await page.waitForLoadState('networkidle')
    await expect(page.getByRole('heading', { name: '履歴書・ES のレビュー' })).toBeVisible({ timeout: 8000 })
    // 入力の流れ（書類 → 応募先 → 結果）が見え、書類が無いうちは開始できない
    await expect(page.getByRole('heading', { name: /レビューする書類/ })).toBeVisible()
    await expect(page.getByRole('button', { name: 'レビューを始める' })).toBeDisabled()
  })

  test('PDFアップロードからレビュー完了まで', async ({ page }) => {
    await page.route('**/api/resume/annotated*', async (route) => {
      await route.fulfill({ status: 404 })
    })

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
        { id: 1, page_number: 1, severity: 'info', message: '具体的な数値を追加すると改善されます', suggestion: '売上10%増などの数値を記載してください', quote: '売上を伸ばしました', rewrite: '店舗売上を前年比10%伸ばしました' },
        { id: 2, page_number: 2, severity: 'warning', message: '成果の記述が不足しています', suggestion: '担当プロジェクトの成果を追記してください', quote: '開発を担当しました', rewrite: '3名でWebの機能開発を1年担当しました' },
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
    await page.waitForLoadState('networkidle')
    await expect(page.getByRole('heading', { name: '履歴書・ES のレビュー' })).toBeVisible({ timeout: 8000 })

    // ファイルを選ぶと、そのまま読み込まれる
    await page.locator('input[type=file]').setInputFiles({ name: '履歴書.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-1.4') })
    await expect(page.getByText('読み込めました')).toBeVisible()

    // #570以降: 企業は候補選択必須。企業を決めないときは「職種だけで見る」で一般レビューする
    const startBtn = page.getByRole('button', { name: 'レビューを始める' })
    await expect(startBtn).toBeDisabled()
    await page.getByRole('radio', { name: '職種だけで見る' }).click()
    await page.getByLabel(/応募職種/).fill('エンジニア')
    await expect(startBtn).toBeEnabled()
    await startBtn.click()

    // 結果: 書類の強調と、番号つきの指摘。選んだ指摘に書き換え案がAIの提案として出る
    await expect(page.getByRole('heading', { name: 'レビュー結果' })).toBeVisible({ timeout: 10000 })
    await expect(page.getByText('全体的にバランスの取れた経歴書です。')).toBeVisible()
    await expect(page.getByText('3名でWebの機能開発を1年担当しました').or(page.getByText('店舗売上を前年比10%伸ばしました'))).toBeVisible()
    await expect(page.getByText(/事実と違う内容になっていないか/)).toBeVisible()
  })
})
