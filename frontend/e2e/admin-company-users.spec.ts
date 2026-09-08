import { test, expect, Page } from '@playwright/test'
import { setupAuth, TEST_ADMIN } from './fixtures/auth'

/**
 * 企業ユーザーの無効化 / 再有効化 (#1196)。
 * 退職者のアクセスを止める唯一の管理画面導線なので、状態表示と PATCH の呼び出しを固定する。
 */

type MockCompanyUser = {
  id: number
  company_id: number
  email: string
  name: string
  role: string
  password_set: boolean
  invite_pending: boolean
  disabled: boolean
  disabled_at: string | null
}

function companyUser(overrides: Partial<MockCompanyUser> & Pick<MockCompanyUser, 'id' | 'email' | 'name'>): MockCompanyUser {
  return {
    company_id: 10,
    role: 'member',
    password_set: true,
    invite_pending: false,
    disabled: false,
    disabled_at: null,
    ...overrides,
  }
}

/** 一覧を差し替え可能な状態として持ち、PATCH 後の再取得で表示が変わることまで確認する */
async function mockCompanyPage(page: Page, users: MockCompanyUser[]) {
  const state = { users }
  const patchBodies: string[] = []

  await page.route('**/api/admin/companies/10/company-users', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: state.users }),
    })
  })

  await page.route('**/api/admin/companies/10/company-users/*', async (route) => {
    const body = route.request().postData() || ''
    patchBodies.push(body)
    const { disabled } = JSON.parse(body) as { disabled: boolean }
    const userID = Number(route.request().url().split('/').pop())
    state.users = state.users.map((u) =>
      u.id === userID ? { ...u, disabled, disabled_at: disabled ? '2026-09-08T00:00:00Z' : null } : u,
    )
    const updated = state.users.find((u) => u.id === userID)
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: userID,
        company_id: 10,
        email: updated?.email,
        disabled,
        disabled_at: disabled ? '2026-09-08T00:00:00Z' : null,
      }),
    })
  })

  await page.route('**/api/admin/companies/10', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ id: 10, name: 'テスト株式会社', data_status: 'published' }),
    })
  })

  return { patchBodies, state }
}

test.describe('企業ユーザーの無効化', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_ADMIN)
    // window.confirm はデフォルトで dismiss されるため、明示的に承認する
    page.on('dialog', (dialog) => dialog.accept())
  })

  test('無効化済みユーザーは「無効」と表示され「有効」にはならない', async ({ page }) => {
    await mockCompanyPage(page, [
      companyUser({ id: 1, email: 'active@example.com', name: '現職 太郎' }),
      companyUser({ id: 2, email: 'left@example.com', name: '退職 花子', disabled: true, disabled_at: '2026-09-01T00:00:00Z' }),
      companyUser({ id: 3, email: 'invited@example.com', name: '招待中 次郎', password_set: false, invite_pending: true }),
    ])

    await page.goto('/admin/companies/10/info')
    await page.waitForLoadState('networkidle')

    const disabledRow = page.getByRole('row').filter({ hasText: 'left@example.com' })
    await expect(disabledRow.getByText('無効', { exact: true })).toBeVisible({ timeout: 8000 })
    await expect(disabledRow.getByText('有効', { exact: true })).toHaveCount(0)

    // 3状態が出し分けられていること
    await expect(
      page.getByRole('row').filter({ hasText: 'active@example.com' }).getByText('有効', { exact: true }),
    ).toBeVisible()
    await expect(
      page.getByRole('row').filter({ hasText: 'invited@example.com' }).getByText('招待中', { exact: true }),
    ).toBeVisible()
  })

  test('無効化すると PATCH が {disabled:true} で呼ばれ表示が「無効」になる', async ({ page }) => {
    const { patchBodies } = await mockCompanyPage(page, [
      companyUser({ id: 1, email: 'active@example.com', name: '現職 太郎' }),
    ])

    await page.goto('/admin/companies/10/info')
    await page.waitForLoadState('networkidle')

    const row = page.getByRole('row').filter({ hasText: 'active@example.com' })
    await expect(row.getByText('有効', { exact: true })).toBeVisible({ timeout: 8000 })

    const patched = page.waitForRequest(
      (req) => req.method() === 'PATCH' && req.url().includes('/company-users/1'),
    )
    await row.getByRole('button', { name: '無効化' }).click()
    await patched

    expect(patchBodies).toEqual([JSON.stringify({ disabled: true })])
    await expect(row.getByText('無効', { exact: true })).toBeVisible({ timeout: 8000 })
    await expect(row.getByText('有効', { exact: true })).toHaveCount(0)
  })

  test('再有効化すると PATCH が {disabled:false} で呼ばれ表示が「有効」に戻る', async ({ page }) => {
    const { patchBodies } = await mockCompanyPage(page, [
      companyUser({ id: 2, email: 'left@example.com', name: '退職 花子', disabled: true, disabled_at: '2026-09-01T00:00:00Z' }),
    ])

    await page.goto('/admin/companies/10/info')
    await page.waitForLoadState('networkidle')

    const row = page.getByRole('row').filter({ hasText: 'left@example.com' })
    await expect(row.getByText('無効', { exact: true })).toBeVisible({ timeout: 8000 })

    const patched = page.waitForRequest(
      (req) => req.method() === 'PATCH' && req.url().includes('/company-users/2'),
    )
    await row.getByRole('button', { name: '再有効化' }).click()
    await patched

    expect(patchBodies).toEqual([JSON.stringify({ disabled: false })])
    await expect(row.getByText('有効', { exact: true })).toBeVisible({ timeout: 8000 })
  })

  test('PATCH が失敗したらエラーを表示し、状態は変えない', async ({ page }) => {
    await mockCompanyPage(page, [companyUser({ id: 1, email: 'active@example.com', name: '現職 太郎' })])
    // 失敗させるため後から上書き（Playwright は後勝ち）
    await page.route('**/api/admin/companies/10/company-users/*', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: '無効化に失敗しました（サーバーエラー）' }),
      })
    })

    await page.goto('/admin/companies/10/info')
    await page.waitForLoadState('networkidle')

    const row = page.getByRole('row').filter({ hasText: 'active@example.com' })
    await row.getByRole('button', { name: '無効化' }).click()

    await expect(page.getByText('無効化に失敗しました（サーバーエラー）')).toBeVisible({ timeout: 8000 })
    await expect(row.getByText('有効', { exact: true })).toBeVisible()
  })
})
