import { test, expect, type Locator, type Page } from '@playwright/test'
import { setupAuth, TEST_ADMIN, TEST_USER } from './fixtures/auth'

/**
 * #1481: モバイルのタップ領域は高さ44px以上。
 * テキストリンクは幅が広がってよい。アイコンボタンは幅も44px以上。
 */
test.use({ viewport: { width: 390, height: 844 }, hasTouch: true })

async function expectMinBox(locator: Locator, minHeight: number, minWidth?: number) {
  await expect(locator).toBeVisible()
  const box = await locator.boundingBox()
  expect(box).not.toBeNull()
  expect(box!.height).toBeGreaterThanOrEqual(minHeight)
  if (minWidth !== undefined) {
    expect(box!.width).toBeGreaterThanOrEqual(minWidth)
  }
}

test.describe('モバイルのタップ領域（#1481）', () => {
  test('ログイン導線のテキストリンクは高さ44px以上', async ({ page }) => {
    await page.context().clearCookies()

    await page.goto('/login')
    await expectMinBox(page.getByRole('link', { name: 'パスワードをお忘れですか？' }), 44)

    await page.goto('/company-portal/sign-in')
    await expectMinBox(page.getByRole('link', { name: 'パスワードをお忘れですか？' }), 44)

    await page.goto('/forgot-password')
    await expectMinBox(page.getByRole('link', { name: 'ログインページへ戻る' }), 44)

    await page.goto('/reset-password')
    await expectMinBox(page.getByRole('link', { name: 'ログインページへ戻る' }), 44)

    await page.goto('/company-portal/forgot-password')
    await expectMinBox(page.getByRole('link', { name: 'ログインページへ戻る' }), 44)

    // トークンが無くてもリンク自体は出る。ここだけ外すと約20pxへ戻っても気づけない。
    await page.goto('/company-portal/reset-password')
    await expectMinBox(page.getByRole('link', { name: 'ログインページへ戻る' }), 44)
  })

  test('企業登録のプライバシーポリシーは高さ44px以上', async ({ page }) => {
    await page.context().clearCookies()
    await page.goto('/company-entry')
    await expectMinBox(page.getByRole('link', { name: 'プライバシーポリシー' }), 44)
  })

  test('履歴書の戻るボタンは44x44以上', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await page.goto('/resume')
    await expectMinBox(page.getByRole('button', { name: '前のページに戻る' }), 44, 44)
  })

  test('面接履歴の期間フィルタは高さ44px以上', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await mockInterviewHistory(page)
    await page.goto('/interview/history')
    for (const name of ['直近5回', '直近10回', '全期間']) {
      await expectMinBox(page.getByRole('button', { name }), 44)
    }
    await expectMinBox(page.getByRole('button', { name: '面接画面へ戻る' }), 44, 44)
  })

  test('予定表の戻るボタンは44x44以上', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await page.route('**/api/schedule*', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      }),
    )
    await page.goto('/schedule')
    await expectMinBox(page.getByRole('button', { name: '前のページに戻る' }), 44, 44)
  })

  test('管理画面ヘッダーの戻るボタンは44x44以上', async ({ page }) => {
    await setupAuth(page, TEST_ADMIN)
    await page.route('**/api/admin/me/school-access*', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ restricted: false, schools: [] }),
      }),
    )
    await page.route('**/api/admin/companies*', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ companies: [] }),
      }),
    )
    await page.route('**/api/admin/teacher/students/tendency-analysis*', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ students: [], total: 0 }),
      }),
    )
    await page.goto('/admin')
    // IconButton に Link を渡しているので button ではなく link になる。
    await expectMinBox(page.getByRole('main').getByRole('link', { name: '戻る' }), 44, 44)
  })
})

async function mockInterviewHistory(page: Page) {
  await page.route('**/api/bff/interviews/trend*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ points: [] }),
    }),
  )
  await page.route('**/api/bff/interviews?*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ sessions: [], total: 0 }),
    }),
  )
}

// CI ではバックエンドが起動していない。未モックの API は空応答で即座に返す（profile-layout.spec.ts と同じ）。
async function stubApi(page: Page) {
  await page.route('**/api/**', (route) =>
    route.request().method() === 'GET'
      ? route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
      : route.fulfill({ status: 204, body: '' }),
  )
}

/** Checkbox・Radio は input が見えないので、押せる範囲（MUI の根の span）を測る。 */
function controlRoot(input: Locator, className: 'MuiCheckbox-root' | 'MuiRadio-root'): Locator {
  return input.locator(`xpath=ancestor::span[contains(concat(' ', @class, ' '), ' ${className} ')][1]`)
}

// #2214: 学生のテーマで押せる範囲を 44px 以上にしたもの（IconButton・ToggleButton・Checkbox・Radio・押せる Chip）を、
// 実際の大きさで測る。単体テスト（jsdom）はレイアウトを計算しないので、指定が付いていることまでしか確かめられない（#2624 のレビュー）。
test.describe('学生画面の押せる範囲（#2214）', () => {
  test('企業登録のプライバシーポリシーの同意のチェックボックスは 44x44 以上', async ({ page }) => {
    await page.context().clearCookies()
    await page.goto('/company-entry')
    const label = page.locator('label').filter({ hasText: 'プライバシーポリシー' })
    await expectMinBox(controlRoot(label.getByRole('checkbox'), 'MuiCheckbox-root'), 44, 44)
  })

  test('プロフィールの画面の見え方の切り替え（Radio）は 44x44 以上', async ({ page }) => {
    await stubApi(page)
    await setupAuth(page, TEST_USER)
    await page.goto('/profile')
    for (const name of ['見やすい表示（推奨）', '高コントラスト（色弱・弱視向け）']) {
      await expectMinBox(controlRoot(page.getByRole('radio', { name }), 'MuiRadio-root'), 44, 44)
    }
  })

  test('ES添削の質問種別（押せる Chip）は高さ 44px 以上', async ({ page }) => {
    await stubApi(page)
    await setupAuth(page, TEST_USER)
    await page.goto('/es-rewrite')
    for (const name of ['志望動機', '自己PR']) {
      await expectMinBox(page.getByRole('button', { name, exact: true }), 44)
    }
  })

  test('企業相関図の切り替え（ToggleButton）は 44x44 以上', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    const json = (body: unknown) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
    const a = { id: 1, name: '親ホールディングス' }
    const b = { id: 2, name: '子テック' }
    await page.route('**/api/companies/relations*', (route) =>
      route.fulfill(json([{ id: 10, parent_id: 1, child_id: 2, relation_type: 'capital_subsidiary', ratio: 60, description: '', parent: a, child: b }])),
    )
    await page.route('**/api/companies/market-info*', (route) => route.fulfill(json([])))
    await page.route(/\/api\/companies\/\d+/, (route) => route.fulfill(json({ id: 2, name: b.name })))
    await page.goto('/correlation-diagram?company_id=2')
    for (const name of ['資本関連図', 'ビジネス関連図']) {
      await expectMinBox(page.getByRole('button', { name, exact: true }), 44, 44)
    }
  })
})
