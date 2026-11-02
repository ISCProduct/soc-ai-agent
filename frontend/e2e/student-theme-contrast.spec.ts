import { test, expect, type Page } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

const STORAGE_KEY = 'student-theme-mode'

async function useMode(page: Page, mode: 'comfortable' | 'high-contrast') {
  await page.addInitScript(([k, v]) => window.localStorage.setItem(k, v), [STORAGE_KEY, mode])
}

/** 要素の実際の文字色・地の色（計算済み）。 */
async function colorsOf(page: Page, text: string) {
  const el = page.getByText(text, { exact: true }).first()
  await expect(el).toBeVisible({ timeout: 15000 })
  return el.evaluate((node) => {
    const cs = getComputedStyle(node)
    return { color: cs.color }
  })
}

// #1571 / #1572: 色の直書きをテーマ由来のトークンへ寄せた。
// 高コントラストを選んだら、テーマを経由していなかった画面の文字色も実際に変わること。
test.describe('学生画面の配色トークン', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)
  })

  test('ES添削: 高コントラストで見出しが純黒になる', async ({ page }) => {
    await page.goto('/es-rewrite')
    expect((await colorsOf(page, 'ES添削・リライト')).color).toBe('rgb(15, 23, 42)')

    await useMode(page, 'high-contrast')
    await page.goto('/es-rewrite')
    // 表示設定はマウント後に localStorage を読んで切り替わる。
    // 本文が見えた直後に読むと、切替前の色で落ちることがある。
    await expect.poll(async () => (await colorsOf(page, 'ES添削・リライト')).color).toBe('rgb(0, 0, 0)')
  })

  test('AI面接: 高コントラストで本文色と地の色が切り替わる', async ({ page }) => {
    await useMode(page, 'high-contrast')
    await page.goto('/interview')
    await expect(page.locator('body')).toBeVisible()
    // 同上。CSS変数は切替前だと既定の #0f172a / #f4f7fb のまま読める。
    await expect.poll(async () => page.evaluate(() => {
      const cs = getComputedStyle(document.documentElement)
      return { ink: cs.getPropertyValue('--ui-ink').trim(), paper: cs.getPropertyValue('--ui-paper').trim() }
    })).toEqual({ ink: '#000000', paper: '#ffffff' })
  })

  test('地の色はテーマ1箇所から配られ、既定は #f4f7fb', async ({ page }) => {
    await page.goto('/es-rewrite')
    await expect(page.getByText('ES添削・リライト', { exact: true })).toBeVisible({ timeout: 15000 })
    const paper = await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue('--ui-paper').trim(),
    )
    const body = await page.evaluate(() => getComputedStyle(document.body).backgroundColor)
    expect(paper).toBe('#f4f7fb')
    expect(body).toBe('rgb(244, 247, 251)')
  })
})
