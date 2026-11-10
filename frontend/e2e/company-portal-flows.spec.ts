import { test, expect } from '@playwright/test'
import { TEST_COMPANY_NAME, mockCompanyPortalApi, setupCompanyAuth } from './fixtures/company'

/**
 * 企業ポータルの中核の業務フロー（SOCAIAGENT-591）。
 * 求人・応募・スカウト・学生検索の主要な導線を、バックエンドをモックして回帰テストで守る。
 * どれも「画面の操作 → 送ったリクエストの中身 → 応答を受けた画面の変化」までを確かめる。
 */

test.beforeEach(async ({ page }) => {
  await setupCompanyAuth(page)
})

test('求人を作って公開する', async ({ page }) => {
  type Job = { id: number; title: string; work_location: string; data_status: string; is_active: boolean }
  const state: { jobs: Job[] } = { jobs: [] }
  const created: Record<string, unknown>[] = []
  const published: Record<string, unknown>[] = []

  const api = await mockCompanyPortalApi(page, {
    'GET /jobs': () => ({ body: { jobs: state.jobs, company_published: true } }),
    'POST /jobs': (req) => {
      const input = req.postDataJSON() as Record<string, unknown>
      created.push(input)
      const job: Job = {
        id: 1,
        title: String(input.title),
        work_location: String(input.work_location ?? ''),
        data_status: 'draft',
        is_active: true,
      }
      state.jobs = [job]
      return { status: 201, body: job }
    },
    'POST /jobs/1/publish': (req) => {
      const input = req.postDataJSON() as { published: boolean }
      published.push(input)
      state.jobs = state.jobs.map((j) => ({ ...j, data_status: input.published ? 'published' : 'draft' }))
      return { body: { job: state.jobs[0], company_published: true } }
    },
  })

  await page.goto('/company-portal/jobs')
  await expect(page.getByText('求人がまだありません')).toBeVisible()

  await page.getByRole('button', { name: '求人を作る' }).first().click()
  const dialog = page.getByRole('dialog', { name: '求人を作る' })
  await dialog.getByLabel('職種名').fill('バックエンドエンジニア')
  await dialog.getByLabel('勤務地').fill('東京都')
  await dialog.getByRole('button', { name: '保存' }).click()

  await expect.poll(() => created.length).toBe(1)
  expect(created[0]).toMatchObject({ title: 'バックエンドエンジニア', work_location: '東京都' })

  const row = page.getByRole('row', { name: /バックエンドエンジニア/ })
  await expect(row.getByText('下書き')).toBeVisible()

  await row.getByRole('button', { name: '公開する' }).click()
  await expect.poll(() => published).toEqual([{ published: true }])
  await expect(row.getByText('公開中')).toBeVisible()
  await expect(row.getByRole('button', { name: '非公開にする' })).toBeVisible()

  expect(api.unexpected).toEqual([])
})

test('応募者のステータスを次の段階へ進める', async ({ page }) => {
  type App = { id: number; user_id: number; student_name: string; status: string; notes: string; created_at: string }
  const state: { apps: App[] } = {
    apps: [
      { id: 11, user_id: 201, student_name: '山田 花子', status: 'applied', notes: '', created_at: '2026-11-01T00:00:00Z' },
    ],
  }
  const patched: { url: string; body: Record<string, unknown> }[] = []

  const api = await mockCompanyPortalApi(page, {
    'GET /dashboard': () => ({
      body: {
        pending_applications: state.apps.length,
        published_jobs: 1,
        new_candidates: 0,
        new_candidate_window_days: 7,
        status_counts: Object.fromEntries(state.apps.map((a) => [a.status, 1])),
      },
    }),
    'GET /applications': () => ({ body: { applications: state.apps, total: state.apps.length, limit: 20, offset: 0 } }),
    'PATCH /applications/*': (req, url) => {
      const body = req.postDataJSON() as { status: string }
      patched.push({ url: url.pathname, body })
      state.apps = state.apps.map((a) => (a.id === 11 ? { ...a, status: body.status } : a))
      return { body: state.apps[0] }
    },
  })

  await page.goto('/company-portal/applications')
  const row = page.getByRole('row', { name: /山田 花子/ })
  await expect(row).toBeVisible()

  // applied から出せるのは「書類選考中」「不採用」
  await expect(row.getByRole('button', { name: '書類選考中' })).toBeVisible()
  await expect(row.getByRole('button', { name: '不採用' })).toBeVisible()

  await row.getByRole('button', { name: '書類選考中' }).click()
  await expect.poll(() => patched.length).toBe(1)
  expect(patched[0]).toEqual({ url: '/api/company-portal/applications/11/status', body: { status: 'document_screening' } })

  // 再取得した一覧では、次の段階のボタンに変わる
  await expect(row.getByRole('button', { name: '書類通過' })).toBeVisible()
  await expect(row.getByRole('button', { name: '書類選考中' })).toHaveCount(0)

  expect(api.unexpected).toEqual([])
})

test('学生の詳細からスカウトを送る', async ({ page }) => {
  const sent: Record<string, unknown>[] = []

  const api = await mockCompanyPortalApi(page, {
    'GET /students/201': () => ({
      body: { analysis: { user_id: 201, name: '山田 花子', interview_reports: [] }, tags: [] },
    }),
    'GET /tags': () => ({ body: { items: [] } }),
    'GET /scout-templates': () => ({
      body: {
        items: [
          {
            id: 7,
            title: '初回のご連絡',
            body: '{{学生名}}さん、{{企業名}}の採用担当です。',
            created_at: '2026-11-01T00:00:00Z',
            updated_at: '2026-11-01T00:00:00Z',
          },
        ],
      },
    }),
    'GET /scouts/cooldown': () => ({ body: { user_id: 201, remaining_ms: 0, blocked: false } }),
    'POST /scouts': (req) => {
      const body = req.postDataJSON() as Record<string, unknown>
      sent.push(body)
      return { status: 201, body: { id: 1, user_id: 201, message: body.message, status: 'sent', created_at: '2026-11-10T00:00:00Z' } }
    },
  })

  await page.goto('/company-portal/students/201')
  await page.getByRole('button', { name: 'スカウトする' }).click()

  const dialog = page.getByRole('dialog', { name: 'スカウトを送る' })
  // テンプレートに学生名と企業名を差し込んだ文面が見える
  await expect(dialog.getByText(`山田 花子さん、${TEST_COMPANY_NAME}の採用担当です。`)).toBeVisible()

  await dialog.getByRole('button', { name: 'スカウトを送る' }).click()
  await expect.poll(() => sent.length).toBe(1)
  expect(sent[0]).toEqual({
    user_id: 201,
    template_id: 7,
    message: `山田 花子さん、${TEST_COMPANY_NAME}の採用担当です。`,
  })

  await expect(dialog.getByText('山田 花子さんへスカウトを送りました')).toBeVisible()
  // 送った直後は、同じ学生へ続けて送れない
  await expect(dialog.getByRole('button', { name: 'スカウトを送る' })).toBeDisabled()

  expect(api.unexpected).toEqual([])
})

test('学生検索を条件で絞り込む', async ({ page }) => {
  const students = [
    {
      user_id: 201,
      name: '山田 花子',
      school_name: 'テスト専門学校',
      target_level: '新卒',
      avatar_url: '',
      certifications_acquired: '基本情報技術者',
      certifications_in_progress: '',
      desired_location: '東京都',
      desired_industry_id: 3,
      desired_industry_name: 'IT・ソフトウェア',
      tags: [],
    },
    {
      user_id: 202,
      name: '佐藤 次郎',
      school_name: 'テスト専門学校',
      target_level: '新卒',
      avatar_url: '',
      certifications_acquired: '',
      certifications_in_progress: '',
      desired_location: '大阪府',
      desired_industry_id: 4,
      desired_industry_name: '製造',
      tags: [],
    },
  ]
  const queries: URLSearchParams[] = []

  const api = await mockCompanyPortalApi(page, {
    'GET /students': (_req, url) => {
      queries.push(url.searchParams)
      const industry = url.searchParams.get('industry_id')
      const location = url.searchParams.get('location')
      const items = students.filter(
        (s) =>
          (!industry || String(s.desired_industry_id) === industry) &&
          (!location || s.desired_location.includes(location)),
      )
      return { body: { items, total: items.length } }
    },
    'GET /tags': () => ({ body: { items: [] } }),
    'GET /industries': () => ({
      body: {
        items: [
          { id: 3, name: 'IT・ソフトウェア', level: 1 },
          { id: 4, name: '製造', level: 1 },
        ],
      },
    }),
  })

  await page.goto('/company-portal/students')
  await expect(page.getByText('山田 花子')).toBeVisible()
  await expect(page.getByText('佐藤 次郎')).toBeVisible()

  await page.getByLabel('希望業界').click()
  await page.getByRole('option', { name: 'IT・ソフトウェア' }).click()
  await page.getByLabel('希望勤務地').fill('東京')
  await page.getByRole('button', { name: '検索' }).click()

  await expect.poll(() => queries.length).toBeGreaterThanOrEqual(2)
  const last = queries[queries.length - 1]
  expect(last.get('industry_id')).toBe('3')
  expect(last.get('location')).toBe('東京')

  await expect(page.getByText('山田 花子')).toBeVisible()
  await expect(page.getByText('佐藤 次郎')).toHaveCount(0)

  await page.getByRole('button', { name: '条件をクリア' }).click()
  await expect(page.getByLabel('希望勤務地')).toHaveValue('')

  expect(api.unexpected).toEqual([])
})
