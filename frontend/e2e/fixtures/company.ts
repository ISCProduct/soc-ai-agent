import { Page, Request } from '@playwright/test'
import { testJwt } from './auth'

/**
 * 企業ポータルの E2E 用フィクスチャ（SOCAIAGENT-591）。
 *
 * 企業ユーザーのログイン状態は、学生とは別の仕組み（company_user_token の Cookie と、
 * sessionStorage の company_user / company_token）で持つ。トークンは期限付きの JWT にする
 * （期限が読めないと ensureFreshToken が取り直しに行き、未ログイン扱いになる）。
 */

export type MockCompanyUser = {
  company_user_id: number
  company_id: number
  email: string
  name: string
  role: 'owner' | 'member'
}

export const TEST_COMPANY_OWNER: MockCompanyUser = {
  company_user_id: 501,
  company_id: 10,
  email: 'owner@example.co.jp',
  name: '採用 太郎',
  role: 'owner',
}

export const TEST_COMPANY_NAME = 'テスト株式会社'

export async function setupCompanyAuth(page: Page, user: MockCompanyUser = TEST_COMPANY_OWNER) {
  const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000'
  const token = testJwt(user.company_user_id)

  await page.context().addCookies([
    { name: 'company_user_id', value: String(user.company_user_id), url: baseURL, httpOnly: true, sameSite: 'Lax' },
    { name: 'company_user_token', value: token, url: baseURL, httpOnly: true, sameSite: 'Lax' },
  ])

  await page.addInitScript(
    ({ u, t }: { u: MockCompanyUser; t: string }) => {
      sessionStorage.setItem('company_user', JSON.stringify(u))
      sessionStorage.setItem('company_token', t)
    },
    { u: user, t: token },
  )

  // ログイン中のユーザー（画面は role で owner 向けの操作を出し分ける）
  await page.route('**/api/company-auth/me', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...user, token }) }),
  )
  // fetchMe のあとに Cookie を張り直す呼び出しと、トークンの取り直し
  await page.route('**/api/company-auth/session', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ company_user_token: token }) }),
  )
}

type MockResponse = { status?: number; body?: unknown }
type PortalHandler = (req: Request, url: URL) => MockResponse | Promise<MockResponse>

/**
 * /api/company-portal/* をまとめて差し替える。キーは "GET /jobs" のように「メソッド + パス」。
 * パスの末尾が * のキーは前方一致（例: "PATCH /applications/*"）。
 * 登録していない呼び出しはテストを失敗させる（実際のバックエンドへ流さない）。
 */
export async function mockCompanyPortalApi(page: Page, handlers: Record<string, PortalHandler>) {
  const unexpected: string[] = []
  const all: Record<string, PortalHandler> = {
    // 共通の枠（CompanyPortalFrame）が企業名と審査状態を取る
    'GET /company': () => ({
      body: {
        id: 10,
        name: TEST_COMPANY_NAME,
        description: '',
        industry: 'IT',
        location: '東京都',
        website_url: '',
        logo_url: '',
        founded_year: 2000,
        employee_count: 100,
        culture: '',
        work_style: '',
        welfare_details: '',
        main_business: '',
        data_status: 'published',
        is_active: true,
        corporate_number: '',
      },
    }),
    ...handlers,
  }

  await page.route('**/api/company-portal/**', async (route) => {
    const req = route.request()
    const url = new URL(req.url())
    const path = url.pathname.replace(/^\/api\/company-portal/, '')
    const key = `${req.method()} ${path}`
    const handler =
      all[key] ??
      Object.entries(all).find(([k]) => k.endsWith('*') && key.startsWith(k.slice(0, -1)))?.[1]
    if (!handler) {
      unexpected.push(key)
      await route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"unmocked"}' })
      return
    }
    const res = await handler(req, url)
    await route.fulfill({
      status: res.status ?? 200,
      contentType: 'application/json',
      body: res.body === undefined ? '' : JSON.stringify(res.body),
    })
  })

  return {
    /** モックしていない呼び出し（テストの最後に空であることを確かめる） */
    unexpected,
  }
}
