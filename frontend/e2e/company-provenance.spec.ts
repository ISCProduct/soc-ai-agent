import { test, expect, type Page } from '@playwright/test'
import { setupAuth } from './fixtures/auth'

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
  // /company/[id] は requireSessionUser() で保護されており、
  // 未ログインだと /login へリダイレクトしてバッジまで到達しない
  test.beforeEach(async ({ page }) => {
    await setupAuth(page)
  })

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

  test('gBizinfo 由来だけの情報は公式情報として表示される', async ({ page }) => {
    await stubCompany(page, {
      source_type: 'gbizinfo',
      corporate_number: '7010401026738',
    })

    await page.goto('/company/42')

    await expect(page.getByTestId('provenance-official').first()).toContainText('公式情報')
    // AI 由来が混ざっていないので AI 推定バッジは出ない
    await expect(page.getByTestId('provenance-ai')).toHaveCount(0)
  })

  // 本番相当DBに実在する "gbizinfo+web_search"（34社）。等値比較ではどの分岐にも
  // 当たらず、AI補完済みの情報が無警告で表示されていた
  test('公的DBとAIが混在する企業はAI推定として警告する', async ({ page }) => {
    await stubCompany(page, {
      source_type: 'gbizinfo+web_search',
      corporate_number: '7010401026738',
    })

    await page.goto('/company/42')

    await expect(page.getByTestId('provenance-ai').first()).toContainText('AI推定')
    await expect(page.getByTestId('provenance-official')).toHaveCount(0)
  })

  // 無言だと「出どころが確かな情報」と同じ見た目になる。この機能の目的が達成できない
  test('出どころが無い企業は「出典不明」として表示する', async ({ page }) => {
    await stubCompany(page, { source_type: '', last_fetch_confidence: '' })

    await page.goto('/company/42')

    await expect(page.getByRole('heading', { name: 'テスト株式会社' })).toBeVisible()
    await expect(page.getByTestId('provenance-unknown').first()).toContainText('出典不明')
    await expect(page.getByTestId('provenance-ai')).toHaveCount(0)
    await expect(page.getByTestId('provenance-official')).toHaveCount(0)
  })
})
