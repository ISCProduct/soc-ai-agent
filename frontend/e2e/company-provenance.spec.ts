import { test, expect, type Page } from '@playwright/test'

/**
 * 企業情報の出どころ表示（#1125 フェーズ1）。
 *
 * AI 推定の情報が公的情報と見分けがつかない状態を解消するのが目的なので、
 * 「AI推定バッジが出る」「根拠リンクが開ける」「公的DB由来では公式情報になる」
 * を実画面で確認する。
 */

const BASE_COMPANY = {
  id: 42,
  name: 'テスト株式会社',
  industry: '情報通信業',
  location: '東京都',
  description: 'テスト用の企業です。',
  employee_count: 100,
  founded_year: 2000,
  website_url: 'https://example.test',
  tech_stack: JSON.stringify(['Go', 'TypeScript']),
}

async function stubCompany(page: Page, overrides: Record<string, unknown>) {
  await page.route('**/api/companies/42', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ...BASE_COMPANY, ...overrides }),
    })
  })
  // 企業ページが並行で叩く周辺APIは空で返して、テスト対象だけに絞る
  for (const path of [
    '**/api/companies/42/job-positions',
    '**/api/companies/42/relations',
    '**/api/companies/42/market-info',
    '**/api/companies/relations*',
    '**/api/companies/market-info*',
  ]) {
    await page.route(path, async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
    })
  }
}

test.describe('企業情報の出どころ表示', () => {
  test('AI推定の情報にはバッジと確信度、根拠リンクが出る', async ({ page }) => {
    await stubCompany(page, {
      source_type: 'web_search',
      source_url: 'https://example.test/ir',
      last_fetch_confidence: 'medium',
      last_model_used: 'gpt-4o-mini',
      info_fetched_at: '2026-08-22T10:51:50.003Z',
    })

    await page.goto('/company/42')

    const badge = page.getByTestId('provenance-ai').first()
    await expect(badge).toBeVisible()
    await expect(badge).toContainText('AI推定')
    await expect(badge).toContainText('中程度')

    const evidence = page.getByTestId('provenance-evidence-link').first()
    await expect(evidence).toHaveAttribute('href', 'https://example.test/ir')
    // 別タブで開く（学生が企業ページから離脱しない）
    await expect(evidence).toHaveAttribute('target', '_blank')
  })

  test('確信度が低い情報は警告色で強調される', async ({ page }) => {
    await stubCompany(page, {
      source_type: 'web_search',
      source_url: 'https://example.test/ir',
      last_fetch_confidence: 'low',
    })

    await page.goto('/company/42')

    const badge = page.getByTestId('provenance-ai').first()
    await expect(badge).toBeVisible()
    await expect(badge).toContainText('低い')
  })

  test('gBizinfo と同期済みの情報は公式情報として表示される', async ({ page }) => {
    await stubCompany(page, {
      source_type: 'web_search',
      last_fetch_confidence: 'low',
      gbiz_last_synced_at: '2026-09-01T00:00:00Z',
      corporate_number: '7010401026738',
    })

    await page.goto('/company/42')

    await expect(page.getByTestId('provenance-official').first()).toContainText('公式情報')
    // 公的DB由来なので AI 推定バッジは出ない
    await expect(page.getByTestId('provenance-ai')).toHaveCount(0)
  })

  test('出どころが無い企業にはバッジを出さない', async ({ page }) => {
    await stubCompany(page, { source_type: '', last_fetch_confidence: '' })

    await page.goto('/company/42')

    await expect(page.getByRole('heading', { name: 'テスト株式会社' })).toBeVisible()
    await expect(page.getByTestId('provenance-ai')).toHaveCount(0)
    await expect(page.getByTestId('provenance-official')).toHaveCount(0)
  })
})
