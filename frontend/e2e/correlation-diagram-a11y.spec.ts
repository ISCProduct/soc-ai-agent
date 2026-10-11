import { test, expect, type Page } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

const A = { id: 1, name: '親ホールディングス' }
const B = { id: 2, name: '子テック' }
const C = { id: 3, name: '取引商事' }

const RELATIONS = [
  { id: 10, parent_id: 1, child_id: 2, relation_type: 'capital_subsidiary', ratio: 60, description: '', parent: A, child: B },
  { id: 11, from_id: 2, to_id: 3, relation_type: 'business_supplier', description: '部品供給', from: B, to: C },
]
const MARKET = [
  { id: 1, company_id: 1, market_type: 'prime', is_listed: true },
  { id: 2, company_id: 2, market_type: 'growth', is_listed: true },
]

async function mockDiagram(page: Page) {
  await setupAuth(page, TEST_USER)
  const json = (body: unknown) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
  await page.route('**/api/companies/relations*', (route) => route.fulfill(json(RELATIONS)))
  await page.route('**/api/companies/market-info*', (route) => route.fulfill(json(MARKET)))
  // 企業情報パネルの中身はこのテストの対象外。止まらないよう空で返す
  await page.route(/\/api\/companies\/\d+/, (route) => route.fulfill(json({ id: 2, name: B.name })))
}

// #1614: 相関図を見られない・マウスを使えない利用者にも同じ情報が届く。
test.describe('企業相関図のアクセシビリティ', () => {
  test('凡例と、図と同じ内容の文章の一覧を出す', async ({ page }) => {
    await mockDiagram(page)
    await page.goto('/correlation-diagram?company_id=2')

    const graph = page.getByRole('group', { name: /子テックを起点とした資本関連図。2社・1件の関係/ })
    await expect(graph).toBeVisible({ timeout: 20000 })
    const legend = page.getByRole('region', { name: '凡例: 線と枠の意味' })
    await expect(legend).toContainText('子会社（実線')
    await expect(legend).toContainText('起点の企業')

    await page.getByRole('button', { name: /関係を一覧で見る（2社・1件の関係）/ }).click()
    const list = page.getByRole('region', { name: /関係の一覧/ })
    await expect(list).toContainText('親ホールディングス → 子テック：子会社（60%）')

    await page.getByRole('button', { name: 'ビジネス関連図' }).click()
    await expect(page.getByRole('region', { name: '凡例: 線と枠の意味' })).toContainText('取引（青線')
    await expect(page.getByRole('region', { name: /関係の一覧/ })).toContainText('子テック → 取引商事：取引（部品供給')
  })

  test('キーボードだけで企業の情報を開ける', async ({ page }) => {
    await mockDiagram(page)
    await page.goto('/correlation-diagram?company_id=2')
    await expect(page.getByRole('group', { name: /資本関連図/ })).toBeVisible({ timeout: 20000 })

    // 一覧の行のボタンから
    const toggle = page.getByRole('button', { name: /関係を一覧で見る/ })
    await toggle.focus()
    await page.keyboard.press('Enter')
    const open = page.getByRole('button', { name: '親ホールディングスの情報' })
    await open.focus()
    await page.keyboard.press('Enter')
    const close = page.getByRole('button', { name: '企業情報を閉じる' })
    await expect(close).toBeVisible()
    await close.click()
    await expect(close).toHaveCount(0)

    // 図のノードへフォーカスして Enter
    const node = page.locator('.react-flow__node[aria-label^="親ホールディングス"]')
    await expect(node).toHaveAttribute('aria-label', /プライム|起点/)
    await node.focus()
    await page.keyboard.press('Enter')
    await expect(page.getByRole('button', { name: '企業情報を閉じる' })).toBeVisible()
  })

  test('動きを減らす設定では取引の線を流さない', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await mockDiagram(page)
    await page.goto('/correlation-diagram?company_id=2')
    await page.getByRole('button', { name: 'ビジネス関連図' }).click({ timeout: 20000 })
    await expect(page.locator('.react-flow__edge')).toHaveCount(1)
    await expect(page.locator('.react-flow__edge.animated')).toHaveCount(0)
  })

  test('通常は取引の線を流して向きを示す', async ({ page }) => {
    await mockDiagram(page)
    await page.goto('/correlation-diagram?company_id=2')
    await page.getByRole('button', { name: 'ビジネス関連図' }).click({ timeout: 20000 })
    await expect(page.locator('.react-flow__edge.animated')).toHaveCount(1)
  })
})
