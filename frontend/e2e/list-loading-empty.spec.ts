import { test, expect, type Page, type Route } from '@playwright/test'
import { setupAuth, TEST_ADMIN, TEST_USER } from './fixtures/auth'

/** 応答を止めておき、Skeleton を確認してから返す。 */
function holdThenFulfill(page: Page, url: string, body: unknown) {
  let release: () => void = () => {}
  const released = new Promise<void>((r) => { release = r })
  void page.route(url, async (route: Route) => {
    await released
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
  })
  return release
}

// #1574: 一覧の読み込みは Skeleton、空のときは「なぜ無いか」と「次の操作」を出す。
test.describe('一覧の Loading / Empty', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)
  })

  test('選考管理: 読み込み中は Skeleton、空なら理由とおすすめ企業への導線', async ({ page }) => {
    const release = holdThenFulfill(page, '**/api/applications', [])
    await page.goto('/applications')
    await expect(page.getByRole('status', { name: '応募を読み込み中' })).toBeVisible({ timeout: 15000 })
    // 見出しは読み込み中から出ている（レイアウトが跳ねない）
    await expect(page.getByRole('heading', { name: '選考管理' })).toBeVisible()

    release()
    await expect(page.getByText('まだ応募した企業はありません')).toBeVisible()
    await expect(page.getByRole('button', { name: 'おすすめ企業を見る' })).toBeVisible()
    await expect(page.getByRole('status', { name: '応募を読み込み中' })).toHaveCount(0)
  })

  test('チャット履歴: 空ならAI相談への導線を出す', async ({ page }) => {
    const release = holdThenFulfill(page, '**/api/chat/sessions', [])
    await page.goto('/chat-history')
    await expect(page.getByRole('status', { name: 'チャット履歴を読み込み中' })).toBeVisible({ timeout: 15000 })
    release()
    await expect(page.getByText('まだチャット履歴がありません')).toBeVisible()
    await page.getByRole('button', { name: 'AIに相談する' }).click()
    await expect(page).toHaveURL(/\/$/)
  })

  test('スケジュール: 読み込み前に「予定なし」を出さない', async ({ page }) => {
    const release = holdThenFulfill(page, '**/api/schedule', [])
    await page.goto('/schedule')
    await expect(page.getByRole('status', { name: '予定を読み込み中' })).toBeVisible({ timeout: 15000 })
    await expect(page.getByText('この月の予定はまだありません')).toHaveCount(0)
    release()
    await expect(page.getByText('この月の予定はまだありません')).toBeVisible()
    await expect(page.getByRole('button', { name: '予定を追加' })).toBeVisible()
  })

  test('面接履歴: 空ならAI面接を始める導線を出す', async ({ page }) => {
    await page.route('**/api/interviews/trend*', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ points: [] }) }),
    )
    const release = holdThenFulfill(page, '**/api/interviews?*', { sessions: [], total: 0 })
    await page.goto('/interview/history')
    await expect(page.getByRole('status', { name: '面接履歴を読み込み中' })).toBeVisible({ timeout: 15000 })
    release()
    await expect(page.getByText('まだ面接の練習記録がありません')).toBeVisible()
    await expect(page.getByRole('button', { name: 'AI面接を始める' })).toBeVisible()
  })
})

test.describe('管理画面一覧の Loading / Empty', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_ADMIN)
    await page.route('**/api/admin/me/school-access*', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ restricted: false, schools: [] }) }),
    )
  })

  test('応募管理: 絞り込み中の空と、そもそも空を言い分ける', async ({ page }) => {
    await page.route('**/api/admin/applications*', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ applications: [] }) }),
    )
    await page.goto('/admin/applications')
    await expect(page.getByText('まだ応募はありません')).toBeVisible({ timeout: 15000 })

    await page.getByRole('combobox').first().click()
    await page.getByRole('option').nth(1).click()
    await expect(page.getByText('この条件に当てはまる応募はありません')).toBeVisible()
    await expect(page.getByText('ステータスを「すべて」に戻すと')).toBeVisible()
  })

  test('応募管理: 絞り込みを素早く変えても、後着の古い応答で一覧を上書きしない', async ({ page }) => {
    let releaseFirst: () => void = () => {}
    const firstHeld = new Promise<void>((r) => { releaseFirst = r })
    await page.route('**/api/admin/applications*', async (route) => {
      const url = new URL(route.request().url())
      if (!url.searchParams.get('status')) {
        await firstHeld
        return route.fulfill({
          status: 200, contentType: 'application/json',
          body: JSON.stringify({ applications: [{ id: 1, status: 'applied', notes: '', user: { name: '古い応答の学生' }, company: { name: 'A社' } }] }),
        })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ applications: [] }) })
    })
    await page.goto('/admin/applications')
    await page.getByRole('combobox').first().click({ timeout: 15000 })
    await page.getByRole('option').nth(1).click()
    await expect(page.getByText('この条件に当てはまる応募はありません')).toBeVisible()

    releaseFirst()
    await page.waitForTimeout(500)
    await expect(page.getByText('古い応答の学生')).toHaveCount(0)
    await expect(page.getByText('この条件に当てはまる応募はありません')).toBeVisible()
  })

  test('就職情報: 取得失敗をエラーとして出し、空表示にしない', async ({ page }) => {
    await page.route('**/api/admin/graduate-employments*', (route) =>
      route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: '就職情報の取得に失敗しました' }) }),
    )
    await page.goto('/admin/graduate-employments')
    await expect(page.getByText('就職情報の取得に失敗しました')).toBeVisible({ timeout: 15000 })
    await expect(page.getByText('まだ就職情報がありません')).toHaveCount(0)
  })
})
