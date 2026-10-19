import { test, expect, type Page } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

function json(body: unknown) {
  return { status: 200, contentType: 'application/json', body: JSON.stringify(body) }
}

async function mockHome(page: Page, opts: { schedule: unknown[]; applications: unknown[]; guidances: unknown[] }) {
  await page.route('**/api/schedule', (route) => route.fulfill(json(opts.schedule)))
  await page.route('**/api/applications', (route) => route.fulfill(json({ applications: opts.applications })))
  await page.route('**/api/user/guidances', (route) => route.fulfill(json({ guidances: opts.guidances })))
  await page.route('**/api/whats-new', (route) => route.fulfill(json([])))
  await page.route('**/api/resume/status', (route) =>
    route.fulfill(json({ has_document: false, latest_score: null, needs_attention: true })),
  )
}

// #1576: ホームで「次にやること」が分かり、お知らせが積み重なってチャットを押し下げない。
test.describe('ホームの次にやること', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)
  })

  test('直近の予定を1件目に出し、残りは件数と操作ボタンで示す', async ({ page }) => {
    const soon = new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString()
    await mockHome(page, {
      schedule: [{ company_name: '株式会社テスト', stage: 'interview', title: '一次面接', scheduled_at: soon }],
      applications: [{ company_name: '内定商事', status: 'offered' }],
      guidances: [],
    })

    await page.goto('/')
    const bar = page.getByRole('region', { name: '次にやること' })
    await expect(bar).toContainText('株式会社テスト 一次面接', { timeout: 15000 })
    await expect(bar).toContainText('（ほか1件）')
    await bar.getByRole('link', { name: '予定を確認する' }).click()
    await expect(page).toHaveURL(/\/schedule$/)
  })

  test('予定が無ければ理由と登録への導線を出す', async ({ page }) => {
    await mockHome(page, { schedule: [], applications: [], guidances: [] })

    await page.goto('/')
    const bar = page.getByRole('region', { name: '次にやること' })
    await expect(bar).toContainText('直近2週間の予定はありません', { timeout: 15000 })
    await expect(bar.getByRole('link', { name: '予定を登録' })).toHaveAttribute('href', '/schedule')
  })

  test('お知らせが複数あっても1件だけ出し、残りは開閉できる', async ({ page }) => {
    await mockHome(page, {
      schedule: [],
      applications: [],
      guidances: [{ id: 1, kind: 'low_match', message: '応募先を一緒に見直しましょう', created_at: '2026-10-01T00:00:00Z' }],
    })

    await page.goto('/')
    await expect(page.getByText('応募先を一緒に見直しましょう')).toBeVisible({ timeout: 15000 })
    const toggle = page.getByRole('button', { name: /ほか\d+件のお知らせを表示/ })
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')
    // 履歴書リマインダは折りたたまれていて見えない
    const resumeLink = page.getByRole('link', { name: '履歴書を確認する' })
    await expect(resumeLink).toBeHidden()

    await toggle.click()
    await expect(page.getByRole('button', { name: 'お知らせを1件だけ表示' })).toHaveAttribute('aria-expanded', 'true')
    await expect(resumeLink).toBeVisible()
  })
})

test.describe('全画面メニュー', () => {
  test('ホーム以外からホームとマッチング結果へ戻れ、現在地が分かる', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await setupAuth(page, TEST_USER)
    await page.route('**/api/schedule', (route) => route.fulfill(json([])))

    await page.goto('/schedule')
    await page.getByRole('button', { name: /メニュー/ }).first().click()
    const nav = page.getByRole('navigation', { name: '画面の切り替え' })
    await expect(nav.getByRole('link', { name: /ホーム（AI相談）/ })).toHaveAttribute('href', '/')
    await expect(nav.getByRole('link', { name: /マッチング結果/ })).toBeVisible()
    const current = nav.locator('[aria-current="page"]')
    await expect(current).toHaveCount(1)
    await expect(current).toContainText('表示中')

    await nav.getByRole('link', { name: /ホーム（AI相談）/ }).click()
    await expect(page).toHaveURL(/\/$/)
  })
})
