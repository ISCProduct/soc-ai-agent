import { test, expect } from '@playwright/test'
import type { ApplicationListItem, CompanyDashboard } from '../lib/company/applications'
import type { CompanyJob } from '../lib/company/jobs'
import type { StudentDetail, StudentListItem } from '../lib/company/students'
import type { CompanyScout, ScoutCooldown, ScoutTemplate } from '../lib/company/scouts'
import { TEST_COMPANY_NAME, TEST_COMPANY_OWNER, mockCompanyPortalApi, setupCompanyAuth } from './fixtures/company'

/**
 * 企業ポータルの中核の業務フロー（SOCAIAGENT-591）。
 * 求人・応募・スカウト・学生検索の主要な導線を、バックエンドをモックして回帰テストで守る。
 * どれも「画面の操作 → 送ったリクエストの中身 → 応答を受けた画面の変化」までを確かめる。
 *
 * モックの応答は、画面が読む型（lib/company/*）で書く。API の形が変わったら型エラーで気づける（#2163 のレビュー）。
 */

const NOW = '2026-11-01T03:00:00Z' // 時差でずれない時刻（UTC でも JST でも 2026/11/1）

function job(overrides: Partial<CompanyJob> & Pick<CompanyJob, 'id' | 'title'>): CompanyJob {
  return {
    description: '',
    job_url: '',
    job_category_id: 0,
    min_salary: 0,
    max_salary: 0,
    employment_type: '',
    work_location: '',
    remote_option: false,
    required_skills: '',
    preferred_skills: '',
    data_status: 'draft',
    is_active: true,
    created_at: NOW,
    updated_at: NOW,
    ...overrides,
  }
}

function application(overrides: Partial<ApplicationListItem> & Pick<ApplicationListItem, 'id' | 'status'>): ApplicationListItem {
  return {
    user_id: 201,
    student_name: '山田 花子',
    notes: '',
    applied_at: NOW,
    status_updated_at: NOW,
    created_at: NOW,
    ...overrides,
  }
}

function dashboard(apps: ApplicationListItem[]): CompanyDashboard {
  return {
    pending_applications: apps.length,
    published_jobs: 1,
    new_candidates: 0,
    new_candidate_window_days: 7,
    status_counts: Object.fromEntries(apps.map((a) => [a.status, 1])),
  }
}

const SCOUT_TEMPLATE: ScoutTemplate = {
  id: 7,
  title: '初回のご連絡',
  body: '{{学生名}}さん、{{企業名}}の採用担当です。',
  created_at: NOW,
  updated_at: NOW,
}

const STUDENT_DETAIL: StudentDetail = {
  analysis: { user_id: 201, name: '山田 花子', interview_reports: [] },
  tags: [],
}

test.describe('owner（管理者）', () => {
  test.beforeEach(async ({ page }) => {
    await setupCompanyAuth(page)
  })

  test('求人を作って公開する', async ({ page }) => {
    const state: { jobs: CompanyJob[] } = { jobs: [] }
    const created: Record<string, unknown>[] = []
    const published: Record<string, unknown>[] = []

    const api = await mockCompanyPortalApi(page, {
      'GET /jobs': () => ({ body: { jobs: state.jobs, company_published: true } }),
      'POST /jobs': (req) => {
        const input = req.postDataJSON() as Record<string, unknown>
        created.push(input)
        const created1 = job({ id: 1, title: String(input.title), work_location: String(input.work_location ?? '') })
        state.jobs = [created1]
        return { status: 201, body: created1 }
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
    await expect(dialog).toBeHidden()

    const row = page.getByRole('row', { name: /バックエンドエンジニア/ })
    await expect(row.getByText('東京都')).toBeVisible()
    await expect(row.getByText('下書き')).toBeVisible()

    await row.getByRole('button', { name: '公開する' }).click()
    await expect.poll(() => published).toEqual([{ published: true }])
    await expect(row.getByText('公開中')).toBeVisible()
    await expect(row.getByRole('button', { name: '非公開にする' })).toBeVisible()

    expect(api.unexpected).toEqual([])
  })

  test('求人の保存に失敗したら、サーバーのメッセージを出し、入力中のダイアログは閉じない', async ({ page }) => {
    const api = await mockCompanyPortalApi(page, {
      'GET /jobs': () => ({ body: { jobs: [], company_published: true } }),
      'POST /jobs': () => ({ status: 500, body: { message: '求人を保存できませんでした（テスト）' } }),
    })

    await page.goto('/company-portal/jobs')
    await page.getByRole('button', { name: '求人を作る' }).first().click()
    const dialog = page.getByRole('dialog', { name: '求人を作る' })
    await dialog.getByLabel('職種名').fill('バックエンドエンジニア')
    await dialog.getByRole('button', { name: '保存' }).click()

    await expect(page.getByText('求人を保存できませんでした（テスト）')).toBeVisible()
    await expect(dialog).toBeVisible()
    await expect(dialog.getByLabel('職種名')).toHaveValue('バックエンドエンジニア')

    expect(api.unexpected).toEqual([])
  })

  test('応募者のステータスを次の段階へ進める', async ({ page }) => {
    const state: { apps: ApplicationListItem[] } = { apps: [application({ id: 11, status: 'applied' })] }
    const patched: { url: string; body: Record<string, unknown> }[] = []

    const api = await mockCompanyPortalApi(page, {
      'GET /dashboard': () => ({ body: dashboard(state.apps) }),
      'GET /applications': () => ({ body: { applications: state.apps, total: state.apps.length, limit: 20, offset: 0 } }),
      'PATCH /applications/*': (req, url) => {
        const body = req.postDataJSON() as { status: string }
        patched.push({ url: url.pathname, body })
        state.apps = state.apps.map((a) =>
          a.id === 11 ? { ...a, status: body.status, status_updated_at: '2026-11-10T03:00:00Z' } : a,
        )
        return { body: state.apps[0] }
      },
    })

    await page.goto('/company-portal/applications')
    const row = page.getByRole('row', { name: /山田 花子/ })
    await expect(row).toBeVisible()
    await expect(row.getByText('応募済み')).toBeVisible()
    await expect(row.getByText('2026/11/1').first()).toBeVisible() // 応募日

    // applied から出せるのは「書類選考中」「不採用」
    await expect(row.getByRole('button', { name: '書類選考中' })).toBeVisible()
    await expect(row.getByRole('button', { name: '不採用' })).toBeVisible()

    await row.getByRole('button', { name: '書類選考中' }).click()
    await expect.poll(() => patched.length).toBe(1)
    expect(patched[0]).toEqual({ url: '/api/company-portal/applications/11/status', body: { status: 'document_screening' } })

    // 再取得した一覧では、ステータスの表示・最終更新日・次の段階のボタンが変わる
    await expect(row.getByText('書類選考中')).toBeVisible()
    await expect(row.getByText('応募済み')).toHaveCount(0)
    await expect(row.getByText('2026/11/10')).toBeVisible()
    await expect(row.getByRole('button', { name: '書類通過' })).toBeVisible()
    await expect(row.getByRole('button', { name: '書類選考中' })).toHaveCount(0)

    expect(api.unexpected).toEqual([])
  })

  test('学生の詳細からスカウトを送る', async ({ page }) => {
    const sent: Record<string, unknown>[] = []

    const api = await mockCompanyPortalApi(page, {
      'GET /students/201': () => ({ body: STUDENT_DETAIL }),
      'GET /tags': () => ({ body: { items: [] } }),
      'GET /scout-templates': () => ({ body: { items: [SCOUT_TEMPLATE] } }),
      'GET /scouts/cooldown': () => ({ body: { user_id: 201, remaining_ms: 0, blocked: false } satisfies ScoutCooldown }),
      'POST /scouts': (req) => {
        const body = req.postDataJSON() as { message: string }
        sent.push(body)
        const scout: CompanyScout = { id: 1, user_id: 201, message: body.message, status: 'sent', created_at: '2026-11-10T03:00:00Z' }
        return { status: 201, body: scout }
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

  test('送信間隔（クールダウン）中の学生にはスカウトを送れない', async ({ page }) => {
    // POST /scouts はモックしない。送ってしまえば unexpected に記録されて落ちる
    const api = await mockCompanyPortalApi(page, {
      'GET /students/201': () => ({ body: STUDENT_DETAIL }),
      'GET /tags': () => ({ body: { items: [] } }),
      'GET /scout-templates': () => ({ body: { items: [SCOUT_TEMPLATE] } }),
      'GET /scouts/cooldown': () => ({
        body: { user_id: 201, remaining_ms: 60 * 60 * 1000, blocked: true } satisfies ScoutCooldown,
      }),
    })

    await page.goto('/company-portal/students/201')
    await page.getByRole('button', { name: 'スカウトする' }).click()

    const dialog = page.getByRole('dialog', { name: 'スカウトを送る' })
    await expect(dialog.getByText('同じ学生への再送は24時間空ける必要があります（クールダウン中）。')).toBeVisible()
    await expect(dialog.getByRole('button', { name: 'スカウトを送る' })).toBeDisabled()

    expect(api.unexpected).toEqual([])
  })

  test('学生検索を条件で絞り込み、条件をクリアして検索し直すと全件に戻る', async ({ page }) => {
    const students: StudentListItem[] = [
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
    // 初回の読み込み（学生・タグ・業界）が終わってから操作する。開発サーバーでは初回の読み込みが2回走り、
    // 開いたメニューの選択肢が描き直されてクリックが外れることがある
    await page.waitForLoadState('networkidle')

    await page.getByLabel('希望業界').click()
    await page.getByRole('option', { name: 'IT・ソフトウェア' }).click()
    await page.getByLabel('希望勤務地').fill('東京')
    // 初回の読み込みは開発サーバーで2回走ることがあるので、回数は決め打ちにせず「押す前より増えたか」で待つ
    const beforeSearch = queries.length
    await page.getByRole('button', { name: '検索' }).click()

    await expect.poll(() => queries.length).toBeGreaterThan(beforeSearch)
    const filtered = queries[queries.length - 1]
    expect(filtered.get('industry_id')).toBe('3')
    expect(filtered.get('location')).toBe('東京')
    await expect(page.getByText('山田 花子')).toBeVisible()
    await expect(page.getByText('佐藤 次郎')).toHaveCount(0)

    // 「条件をクリア」は入力を空に戻すだけで、検索はし直さない（今の仕様）。検索し直すと全件に戻る
    await page.getByRole('button', { name: '条件をクリア' }).click()
    await expect(page.getByLabel('希望勤務地')).toHaveValue('')
    const beforeResearch = queries.length
    await page.getByRole('button', { name: '検索' }).click()

    await expect.poll(() => queries.length).toBeGreaterThan(beforeResearch)
    const cleared = queries[queries.length - 1]
    expect(cleared.get('industry_id')).toBeNull()
    expect(cleared.get('location')).toBeNull()
    await expect(page.getByText('山田 花子')).toBeVisible()
    await expect(page.getByText('佐藤 次郎')).toBeVisible()

    expect(api.unexpected).toEqual([])
  })
})

// member のテストは「出さない」ことを確かめるので、描画が落ち着いてから確かめる。行が出た直後は、
// 権限の判定（fetchMe）を反映した再描画の前のことがあり、そこで「無い」と判定すると通ってしまう
test.describe('member（一般の担当者）', () => {
  test.beforeEach(async ({ page }) => {
    await setupCompanyAuth(page, { ...TEST_COMPANY_OWNER, company_user_id: 502, name: '担当 花子', role: 'member' })
  })

  test('求人の作成・公開のボタンを出さない', async ({ page }) => {
    const api = await mockCompanyPortalApi(page, {
      'GET /jobs': () => ({
        body: { jobs: [job({ id: 1, title: 'バックエンドエンジニア', work_location: '東京都' })], company_published: true },
      }),
    })

    await page.goto('/company-portal/jobs')
    const row = page.getByRole('row', { name: /バックエンドエンジニア/ })
    await expect(row).toBeVisible()
    await page.waitForLoadState('networkidle')
    await expect(page.getByRole('button', { name: '求人を作る' })).toHaveCount(0)
    await expect(row.getByRole('button', { name: '公開する' })).toHaveCount(0)
    await expect(row.getByRole('button', { name: '編集' })).toHaveCount(0)

    expect(api.unexpected).toEqual([])
  })

  test('求人が無いときは、作成は管理者のみと案内する', async ({ page }) => {
    const api = await mockCompanyPortalApi(page, {
      'GET /jobs': () => ({ body: { jobs: [], company_published: true } }),
    })

    await page.goto('/company-portal/jobs')
    await expect(page.getByText('求人の作成は管理者のみ行えます。')).toBeVisible()
    await page.waitForLoadState('networkidle')
    await expect(page.getByRole('button', { name: '求人を作る' })).toHaveCount(0)

    expect(api.unexpected).toEqual([])
  })

  test('応募者のステータスを変えるボタンを出さない', async ({ page }) => {
    const apps = [application({ id: 11, status: 'applied' })]
    const api = await mockCompanyPortalApi(page, {
      'GET /dashboard': () => ({ body: dashboard(apps) }),
      'GET /applications': () => ({ body: { applications: apps, total: apps.length, limit: 20, offset: 0 } }),
    })

    await page.goto('/company-portal/applications')
    const row = page.getByRole('row', { name: /山田 花子/ })
    await expect(row.getByText('応募済み')).toBeVisible()
    await page.waitForLoadState('networkidle')
    await expect(page.getByRole('columnheader', { name: '次の操作' })).toHaveCount(0)
    await expect(row.getByRole('button', { name: '書類選考中' })).toHaveCount(0)
    await expect(row.getByRole('button', { name: '不採用' })).toHaveCount(0)

    expect(api.unexpected).toEqual([])
  })
})
