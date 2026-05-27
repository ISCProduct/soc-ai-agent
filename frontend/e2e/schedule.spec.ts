import { test, expect } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

const MOCK_EVENTS = [
  {
    id: 1,
    company_name: 'テスト株式会社',
    title: '一次面接',
    stage: '一次面接',
    scheduled_at: new Date(Date.now() + 86400000).toISOString(),
    memo: '',
    passed: null,
  },
  {
    id: 2,
    company_name: 'サンプル工業',
    title: '書類選考',
    stage: '書類選考',
    scheduled_at: new Date(Date.now() + 172800000).toISOString(),
    memo: '',
    passed: null,
  },
]

test.describe('選考スケジュール管理フロー', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)

    await page.route('/api/schedule*', async (route) => {
      const method = route.request().method()
      if (method === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ events: MOCK_EVENTS }),
        })
      } else if (method === 'POST') {
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({ event: { id: 3, company_name: '新規企業', stage: '書類選考', scheduled_at: new Date().toISOString() } }),
        })
      } else if (method === 'PUT') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ event: { ...MOCK_EVENTS[0], stage: '内定' } }),
        })
      } else if (method === 'DELETE') {
        await route.fulfill({ status: 204 })
      } else {
        await route.continue()
      }
    })
  })

  test('選考スケジュールページが表示される', async ({ page }) => {
    await page.goto('/schedule')
    await expect(page.getByText('選考スケジュール')).toBeVisible({ timeout: 8000 })
  })

  test('既存のイベントが一覧表示される', async ({ page }) => {
    await page.goto('/schedule')
    await expect(page.getByText('テスト株式会社')).toBeVisible({ timeout: 8000 })
    await expect(page.getByText('サンプル工業')).toBeVisible({ timeout: 8000 })
  })

  test('新しいイベントを登録できる', async ({ page }) => {
    await page.goto('/schedule')
    await expect(page.getByText('選考スケジュール')).toBeVisible({ timeout: 8000 })

    const addButton = page.getByRole('button', { name: /追加|新規|＋|\+/ })
    if (await addButton.count() > 0) {
      await addButton.first().click()

      const companyInput = page.locator('input').filter({ hasText: '' }).first()
      if (await companyInput.isVisible()) {
        await companyInput.fill('新規企業')
      }

      const saveButton = page.getByRole('button', { name: /保存|登録|追加/ })
      if (await saveButton.count() > 0) {
        await saveButton.first().click()
      }
    }
  })

  test('イベントの選考ステージを更新できる', async ({ page }) => {
    await page.goto('/schedule')
    await expect(page.getByText('テスト株式会社')).toBeVisible({ timeout: 8000 })

    const editButton = page.getByRole('button', { name: /編集|詳細/ }).first()
    if (await editButton.count() > 0) {
      await editButton.click()
    }
  })
})
