import { readFileSync } from 'fs'
import { join } from 'path'
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

  test('実際の書類（PDF）を描き、指摘の位置に番号つきの強調を重ねる', async ({ page }) => {
    const pdf = readFileSync(join(__dirname, 'fixtures', 'sample-resume.pdf'))
    await page.route('**/api/resume/document*', (route) =>
      route.fulfill({ status: 200, contentType: 'application/pdf', body: pdf }),
    )
    await page.route('**/api/resume/upload', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ document: { id: 42 } }) }),
    )
    // bbox は OCR と同じ、150dpi の画素（左上が原点）。ページの大きさ 1240x1755 に対する位置
    const item = {"id": 1, "page_number": 1, "severity": "critical", "message": "何を・どれくらい・どう取り組んだかが読み取れません。", "quote": "学生時代はいろいろな活動を頑張りました。", "rewrite": "学園祭の実行委員として【担当した役割】を担い、【数字で表せる成果】を出しました。", "bbox": "{\"bbox\": [125.00000000000001, 433.33333333333337, 625.0, 463.33332061767584], \"page_width\": 1240, \"page_height\": 1755}"}
    await page.route('**/api/resume/review/stream*', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: [
          `data: ${JSON.stringify({ type: 'chunk', text: 'レポート' })}`,
          `data: ${JSON.stringify({ type: 'complete', review: { id: 1, score: 62, summary: '改善の余地があります。' }, items: [item], annotated_available: false })}`,
        ].join('\n') + '\n',
      }),
    )

    await page.goto('/resume')
    await page.waitForLoadState('networkidle')
    await page.locator('input[type=file]').setInputFiles({ name: '履歴書.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-1.4') })
    await expect(page.getByText('読み込めました')).toBeVisible()
    await page.getByRole('radio', { name: '職種だけで見る' }).click()
    await page.getByLabel(/応募職種/).fill('エンジニア')
    await page.getByRole('button', { name: 'レビューを始める' }).click()

    // PDF.js が書類のページを描く（原文を並べた代わりの表示ではない）
    const canvas = page.locator('canvas').first()
    await expect(canvas).toBeVisible({ timeout: 20000 })
    await expect(page.getByText(/実際の書類のレイアウトとは異なります/)).toHaveCount(0)

    // 強調は、ページの大きさに対する割合の位置に重なる（x0=125/1240、y0=433/1755）
    const highlight = page.getByRole('button', { name: /指摘1を選ぶ/ })
    await expect(highlight).toBeVisible()
    const c = (await canvas.boundingBox())!
    const h = (await highlight.boundingBox())!
    expect((h.x - c.x) / c.width).toBeCloseTo(125 / 1240, 1)
    expect((h.y - c.y) / c.height).toBeCloseTo(433 / 1755, 1)
  })
})
