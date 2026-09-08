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
}

// data_available: true の生徒。タイプと業界が出る
const STUDENT_WITH_DATA: TendencyStudent = {
  user_id: 1,
  name: '山田太郎',
  email: 'yamada@example.com',
  type_label: '共感支援タイプ',
  top_categories: [
    { category: '対人志向', score: 90 },
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
    await expect(row.getByText('共感支援タイプ')).toBeVisible()
    await expect(row.getByText('対人志向 90')).toBeVisible()
    await expect(row.getByText('ソフトウェア開発 81.2')).toBeVisible()
    await expect(row.getByText('情報通信業 76.4')).toBeVisible()

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
    await expect(page.getByText('技術志向 88')).toHaveCount(0)
    await expect(page.getByText('金融業 55.5')).toHaveCount(0)

    // スコアがある生徒側は従来通り表示される
    await expect(page.getByRole('row').filter({ hasText: '山田太郎' }).getByText('共感支援タイプ')).toBeVisible()
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
    await expect(page.getByText('生徒が見つかりません')).toBeVisible({ timeout: 8000 })
    await expect(page.getByText('あくまで参考情報')).toBeVisible()
  })
})
