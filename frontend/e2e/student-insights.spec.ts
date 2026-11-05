import { test, expect, type Page } from '@playwright/test'
import { setupAuth, TEST_ADMIN } from './fixtures/auth'

type TendencyStudent = {
  user_id: number
  name: string
  email: string
  type_label: string
  top_categories: { category: string; score: number }[] | null
  suited_industries: { industry_id: number; industry_name: string; score: number }[] | null
  data_available: boolean
  last_activity_at?: string | null
}

// data_available: true の生徒。タイプと業界が出る
const STUDENT_WITH_DATA: TendencyStudent = {
  user_id: 1,
  name: '山田太郎',
  email: 'yamada@example.com',
  type_label: '対話タイプ',
  top_categories: [
    { category: 'コミュニケーション力', score: 90 },
    { category: '成長志向', score: 72 },
  ],
  suited_industries: [
    { industry_id: 2, industry_name: 'ソフトウェア開発', score: 81.2 },
    { industry_id: 1, industry_name: '情報通信業', score: 76.4 },
  ],
  data_available: true,
}

// data_available: false なのにタイプ名が入っているケース。
// 画面が data_available を見ずに type_label をそのまま出すと、このタイプ名が漏れる
const STUDENT_WITHOUT_DATA: TendencyStudent = {
  user_id: 2,
  name: '佐藤花子',
  email: 'sato@example.com',
  type_label: '技術探究タイプ',
  top_categories: [{ category: '技術志向', score: 88 }],
  suited_industries: [{ industry_id: 7, industry_name: '金融業', score: 55.5 }],
  data_available: false,
}

async function mockCommon(page: Page) {
  await setupAuth(page, TEST_ADMIN)
  // SchoolFilterSelect と学校必須判定が呼ぶ。未モックだと Backend 不在でハングする
  await page.route('**/api/admin/me/school-access*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ restricted: false, schools: [] }),
    })
  })
}

async function mockTendency(page: Page, students: TendencyStudent[]) {
  await page.route('**/api/admin/teacher/students/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ students, total: students.length, limit: 25, offset: 0 }),
    })
  })
}

test.describe('生徒の傾向分析', () => {
  test('生徒一覧が表形式で表示される', async ({ page }) => {
    await mockCommon(page)
    await mockTendency(page, [STUDENT_WITH_DATA])

    await page.goto('/admin/student-insights')
    await expect(page.getByRole('heading', { name: '生徒の傾向分析' })).toBeVisible({ timeout: 15000 })

    const row = page.getByRole('row').filter({ hasText: '山田太郎' })
    await expect(row).toBeVisible({ timeout: 8000 })
    await expect(row.getByText('yamada@example.com')).toBeVisible()
    await expect(row.getByText('対話タイプ')).toBeVisible()
    // スコアバーは一覧を縦に伸ばすので詳細に畳む（#1573）。開くと出る
    await expect(page.getByRole('img', { name: 'コミュニケーション力 90点' })).toHaveCount(0)
    await row.getByRole('button', { name: 'スコアを見る' }).click()
    await expect(row.getByRole('button', { name: '閉じる' })).toHaveAttribute('aria-expanded', 'true')
    // スコアはバー表示(#1225)なので、ラベルと数値は別要素。
    // 読み上げ用の aria-label で「どのラベルが何点か」を確認する
    await expect(page.getByRole('img', { name: 'コミュニケーション力 90点' })).toBeVisible()
    await expect(page.getByRole('img', { name: 'ソフトウェア開発 81.2点' })).toBeVisible()
    await expect(page.getByRole('img', { name: '情報通信業 76.4点' })).toBeVisible()

    // 参考情報である旨の注記(教育現場での誤用防止)
    await expect(page.getByText('あくまで参考情報')).toBeVisible()
  })

  test('data_available が false の生徒はタイプ名を出さず「分析データ不足」と表示する', async ({ page }) => {
    await mockCommon(page)
    await mockTendency(page, [STUDENT_WITH_DATA, STUDENT_WITHOUT_DATA])

    await page.goto('/admin/student-insights')

    const noDataRow = page.getByRole('row').filter({ hasText: '佐藤花子' })
    await expect(noDataRow).toBeVisible({ timeout: 15000 })
    await expect(noDataRow.getByText('分析データ不足')).toBeVisible()

    // タイプ名も上位カテゴリも業界も出してはいけない
    await expect(page.getByText('技術探究タイプ')).toHaveCount(0)
    await expect(page.getByRole('img', { name: '技術志向 88点' })).toHaveCount(0)
    await expect(page.getByRole('img', { name: '金融業 55.5点' })).toHaveCount(0)

    // スコアがある生徒側は従来通り表示される
    await expect(page.getByRole('row').filter({ hasText: '山田太郎' }).getByText('対話タイプ')).toBeVisible()
  })

  test('検索するとクエリパラメータ q 付きで API が呼ばれる', async ({ page }) => {
    await mockCommon(page)
    await mockTendency(page, [STUDENT_WITH_DATA])

    await page.goto('/admin/student-insights')
    await expect(page.getByRole('row').filter({ hasText: '山田太郎' })).toBeVisible({ timeout: 15000 })

    const searchRequest = page.waitForRequest((req) => {
      const url = new URL(req.url())
      return url.pathname === '/api/admin/teacher/students/tendency-analysis'
        && url.searchParams.get('q') === '佐藤'
    })
    await page.getByPlaceholder('氏名・メール・学校名で検索').fill('佐藤')
    const request = await searchRequest

    const params = new URL(request.url()).searchParams
    expect(params.get('q')).toBe('佐藤')
    expect(params.get('limit')).toBe('25')
    expect(params.get('offset')).toBe('0')
  })

  test('生徒が0人でも画面が壊れない', async ({ page }) => {
    await mockCommon(page)
    await mockTendency(page, [])

    await page.goto('/admin/student-insights')
    await expect(page.getByRole('heading', { name: '生徒の傾向分析' })).toBeVisible({ timeout: 15000 })
    await expect(page.getByText('担当の生徒がまだいません')).toBeVisible({ timeout: 8000 })
    await expect(page.getByText('あくまで参考情報')).toBeVisible()
  })
})

const DAY = 24 * 60 * 60 * 1000

// #1573: 未活動の学生を教員が見つけられる。文言は事実ベース。
test.describe('未活動の学生', () => {
  test('最終活動の列を出し、14日以上・記録なしの生徒に事実を添える', async ({ page }) => {
    await mockCommon(page)
    await mockTendency(page, [
      { ...STUDENT_WITH_DATA, last_activity_at: new Date(Date.now() - 3 * DAY).toISOString() },
      { ...STUDENT_WITHOUT_DATA, last_activity_at: null },
    ])

    await page.goto('/admin/student-insights')
    const active = page.getByRole('row').filter({ hasText: '山田太郎' })
    await expect(active).toContainText('3日前', { timeout: 15000 })
    await expect(active).not.toContainText('活動がありません')
    const idle = page.getByRole('row').filter({ hasText: '佐藤花子' })
    await expect(idle).toContainText('活動の記録なし')
    await expect(idle).toContainText('最近14日間、活動がありません')
  })

  test('?inactive_days で開くと絞り込み済みで取得し、0件なら絞り込みを外せる', async ({ page }) => {
    await mockCommon(page)
    const seen: URLSearchParams[] = []
    await page.route('**/api/admin/teacher/students/**', async (route) => {
      const params = new URL(route.request().url()).searchParams
      seen.push(params)
      const students = params.has('inactive_days') ? [] : [STUDENT_WITH_DATA]
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ students, total: students.length, limit: 25, offset: 0 }),
      })
    })

    await page.goto('/admin/student-insights?inactive_days=14')
    await expect(page.getByText('条件に合う生徒はいません')).toBeVisible({ timeout: 15000 })
    await expect(page.getByRole('checkbox', { name: '最近14日間、活動がない生徒のみ' })).toBeChecked()
    expect(seen.some((p) => p.get('inactive_days') === '14')).toBe(true)

    await page.getByRole('button', { name: '絞り込みを外す' }).click()
    await expect(page.getByRole('row').filter({ hasText: '山田太郎' })).toBeVisible()
    await expect(page.getByRole('checkbox', { name: '最近14日間、活動がない生徒のみ' })).not.toBeChecked()
  })

  test('タブレット幅で絞り込みが折り返し、横にはみ出さない', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 })
    await mockCommon(page)
    await mockTendency(page, [STUDENT_WITH_DATA])

    await page.goto('/admin/student-insights')
    const lastFilter = page.getByRole('checkbox', { name: '履歴書要対応の生徒のみ' })
    await expect(lastFilter).toBeVisible({ timeout: 15000 })
    const box = await lastFilter.boundingBox()
    expect(box && box.x + box.width).toBeLessThanOrEqual(768)
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow).toBeLessThanOrEqual(0)
  })
})

test.describe('教員の入口', () => {
  test('対応が必要な学生を条件ごとに出し、その条件で一覧を開ける', async ({ page }) => {
    await mockCommon(page)
    await page.route('**/api/admin/companies*', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ companies: [] }) }),
    )
    await page.route('**/api/admin/teacher/students/**', async (route) => {
      const params = new URL(route.request().url()).searchParams
      const students = params.has('inactive_days') ? [STUDENT_WITHOUT_DATA] : []
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ students, total: students.length ? 4 : 0, limit: 3, offset: 0 }),
      })
    })

    await page.goto('/admin')
    const section = page.getByRole('region', { name: '対応が必要な学生' })
    await expect(section).toContainText('4人: 最近14日間、活動がありません', { timeout: 15000 })
    await expect(section).toContainText('佐藤花子 ほか3人')
    // 0人の条件は出さない
    await expect(section).not.toContainText('履歴書が未提出か')

    await section.getByRole('link', { name: '該当する学生を一覧で見る' }).click()
    await expect(page).toHaveURL(/\/admin\/student-insights\?inactive_days=14/, { timeout: 30000 })
    await expect(page.getByRole('checkbox', { name: '最近14日間、活動がない生徒のみ' })).toBeChecked({ timeout: 15000 })
  })

  test('読み込みに失敗したら空ではなくエラーと再読み込みを出す', async ({ page }) => {
    await mockCommon(page)
    await page.route('**/api/admin/companies*', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ companies: [] }) }),
    )
    await page.route('**/api/admin/teacher/students/**', (route) =>
      route.fulfill({ status: 500, contentType: 'application/json', body: '{}' }),
    )

    await page.goto('/admin')
    const section = page.getByRole('region', { name: '対応が必要な学生' })
    await expect(section.getByRole('alert')).toContainText('読み込めませんでした', { timeout: 15000 })
    await expect(section.getByRole('button', { name: '再読み込み' })).toBeVisible()
    await expect(section).not.toContainText('いま当てはまる学生はいません')
  })
})
