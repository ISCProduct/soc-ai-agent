import { test, expect, type Page, type Route } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

const SESSION_ID = '0b7c3b8e-6a39-4d3e-9a51-3f1e0f6f2a10'
const QUESTIONS = [
  { id: 101, position: 1, body: '人と協力して進める仕事が好きだ' },
  { id: 102, position: 2, body: '一人で黙々と作業したい' },
  { id: 103, position: 3, body: '新しいやり方を試すのが好きだ' },
  { id: 104, position: 4, body: '決まった手順のほうが安心する' },
]

type MockOptions = { templates?: number; failAnswers?: boolean; listDelayMs?: number }

/** Backend の代わりに受検状態を持つ。中断→再開で回答が残ることを確かめるため。 */
async function mockBackend(page: Page, opts: MockOptions = {}) {
  const state = {
    status: 'not_started' as 'not_started' | 'in_progress' | 'submitted',
    answers: {} as Record<string, number>,
    puts: [] as { id: string; value: number }[],
    submits: 0,
    // テスト途中から保存を遅らせて失敗させる（選び直しの保存が提出に間に合わないケース）
    slowFailPuts: false,
  }
  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
  const view = () => ({ session_id: SESSION_ID, title: '標準性格検査', status: state.status, questions: QUESTIONS, answers: state.answers })

  await page.route('**/api/assessments**', async (route) => {
    const req = route.request()
    const path = new URL(req.url()).pathname.replace(/^\/api\/assessments/, '')
    if (req.method() === 'GET' && path === '') {
      if (opts.listDelayMs) await new Promise((r) => setTimeout(r, opts.listDelayMs))
      const assessments = Array.from({ length: opts.templates ?? 1 }, () => ({
        template_id: 1,
        title: '標準性格検査',
        description: '仕事への向き方の傾向を見ます',
        question_count: QUESTIONS.length,
        status: state.status,
        session_id: state.status === 'not_started' ? undefined : SESSION_ID,
        answered_count: Object.keys(state.answers).length,
      }))
      return json(route, { assessments })
    }
    if (req.method() === 'POST' && path === '/1/sessions') {
      if (state.status === 'not_started') state.status = 'in_progress'
      return json(route, view())
    }
    if (req.method() === 'GET' && path === `/sessions/${SESSION_ID}`) return json(route, view())
    const put = path.match(new RegExp(`^/sessions/${SESSION_ID}/answers/(\\d+)$`))
    if (req.method() === 'PUT' && put) {
      if (opts.failAnswers) return json(route, { error: 'boom' }, 500)
      if (state.slowFailPuts) {
        await new Promise((r) => setTimeout(r, 800))
        return json(route, { error: 'boom' }, 500)
      }
      const { value } = req.postDataJSON() as { value: number }
      state.answers[put[1]] = value
      state.puts.push({ id: put[1], value })
      return route.fulfill({ status: 204 })
    }
    if (req.method() === 'POST' && path === `/sessions/${SESSION_ID}/submit`) {
      state.submits++
      if (state.status === 'submitted') return json(route, { error: 'この検査はすでに提出済みです' }, 409)
      if (Object.keys(state.answers).length < QUESTIONS.length) return json(route, { error: '未回答' }, 400)
      state.status = 'submitted'
      return json(route, { session_id: SESSION_ID })
    }
    if (req.method() === 'GET' && path === `/sessions/${SESSION_ID}/result`) {
      return json(route, {
        session_id: SESSION_ID,
        title: '標準性格検査',
        scores: [
          { category: 'チームワーク志向', score: 75 },
          { category: '挑戦志向', score: 62.5 },
        ],
      })
    }
    return json(route, { error: 'not mocked' }, 404)
  })
  return state
}

async function choose(page: Page, questionBody: string, label: string) {
  const group = page.getByRole('group', { name: new RegExp(questionBody) })
  await group.getByRole('radio', { name: label, exact: true }).check()
  await expect(group.getByText('✓ 保存しました')).toBeVisible()
}

// #1678: 適性検査の受検（中断→再開→提出→結果）
test.describe('適性検査', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)
  })

  test('途中でやめても回答が残り、再開して提出すると結果と使われ方が出る', async ({ page }) => {
    const state = await mockBackend(page)
    await page.goto('/assessments')
    const card = page.getByRole('listitem').filter({ hasText: '標準性格検査' })
    await expect(card.getByTestId('assessment-status')).toHaveText('○ 未受検', { timeout: 15000 })
    await card.getByRole('button', { name: '受検を始める' }).click()

    await expect(page).toHaveURL(new RegExp(`/assessments/${SESSION_ID}$`))
    await expect(page.getByText('1 / 2 ページ')).toBeVisible({ timeout: 15000 })
    const next = page.getByRole('button', { name: '次へ' })
    await expect(next).toBeDisabled()
    await choose(page, QUESTIONS[0].body, 'あてはまる')

    // キーボードだけでも答えられる（矢印キーで選択が移り、保存される）
    const second = page.getByRole('group', { name: new RegExp(QUESTIONS[1].body) })
    await second.getByRole('radio', { name: 'あてはまらない', exact: true }).focus()
    await page.keyboard.press('Space')
    await page.keyboard.press('ArrowDown')
    await expect(second.getByRole('radio', { name: 'あまりあてはまらない', exact: true })).toBeChecked()
    await expect(second.getByText('✓ 保存しました')).toBeVisible()
    await expect.poll(() => state.answers['102']).toBe(2)

    await next.click()
    await expect(page.getByText('2 / 2 ページ')).toBeVisible()
    await choose(page, QUESTIONS[2].body, 'ややあてはまる')

    // 中断
    await page.getByRole('button', { name: /中断して一覧へ戻る/ }).click()
    await expect(page).toHaveURL(/\/assessments$/)
    await expect(card.getByTestId('assessment-status')).toHaveText('◐ 受検中')
    await expect(card).toContainText('4問中 3問 回答済み')

    // 再開すると未回答のページから始まり、答えた内容が選ばれている
    await card.getByRole('button', { name: '続きから再開する' }).click()
    await expect(page.getByText('2 / 2 ページ')).toBeVisible({ timeout: 15000 })
    await expect(
      page.getByRole('group', { name: new RegExp(QUESTIONS[2].body) }).getByRole('radio', { name: 'ややあてはまる' }),
    ).toBeChecked()
    const submit = page.getByRole('button', { name: '提出する' })
    await expect(submit).toBeDisabled()
    await choose(page, QUESTIONS[3].body, 'どちらともいえない')
    await submit.click()

    await expect(page).toHaveURL(new RegExp(`/assessments/${SESSION_ID}/result$`))
    await expect(page.getByRole('heading', { name: '標準性格検査の結果' })).toBeVisible({ timeout: 15000 })
    const bars = page.getByRole('list', { name: 'カテゴリ別の傾向' })
    await expect(bars.getByRole('progressbar', { name: 'チームワーク志向 75 / 100' })).toBeVisible()
    await expect(bars).toContainText('63')
    await expect(page.getByRole('heading', { name: 'この結果の使われ方' })).toBeVisible()
    await expect(page.getByText(/偏差値|位$/)).toHaveCount(0)

    await page.getByRole('button', { name: '検査の一覧へ戻る' }).click()
    await expect(card.getByTestId('assessment-status')).toHaveText('● 提出済み')
    await card.getByRole('button', { name: '結果を見る' }).click()
    await expect(page).toHaveURL(new RegExp(`/assessments/${SESSION_ID}/result$`))
  })

  test('保存に失敗したら理由と再操作を示し、提出させない', async ({ page }) => {
    await mockBackend(page, { failAnswers: true })
    await page.goto(`/assessments/${SESSION_ID}`)
    const group = page.getByRole('group', { name: new RegExp(QUESTIONS[0].body) })
    await group.getByRole('radio', { name: 'あてはまる', exact: true }).check({ timeout: 15000 })
    await expect(group.getByRole('status')).toContainText('保存できませんでした')
    await expect(group.getByRole('status')).toContainText('答えた内容は保存されています')

    // 画面上は全問選んでいても、保存できていない回答があるうちは提出できない
    await page.getByRole('group', { name: new RegExp(QUESTIONS[1].body) }).getByRole('radio', { name: 'あてはまる', exact: true }).check()
    await page.getByRole('button', { name: '次へ' }).click()
    for (const q of QUESTIONS.slice(2)) {
      await page.getByRole('group', { name: new RegExp(q.body) }).getByRole('radio', { name: 'あてはまる', exact: true }).check()
    }
    await expect(page.getByRole('button', { name: '提出する' })).toBeDisabled()
  })

  test('選び直した回答の保存が失敗したら、古い回答のまま提出しない', async ({ page }) => {
    const state = await mockBackend(page)
    await page.goto(`/assessments/${SESSION_ID}`)
    await choose(page, QUESTIONS[0].body, 'あてはまる')
    await choose(page, QUESTIONS[1].body, 'あてはまる')
    await page.getByRole('button', { name: '次へ' }).click()
    await choose(page, QUESTIONS[2].body, 'あてはまる')
    await choose(page, QUESTIONS[3].body, 'あてはまる')

    // 全問保存済みの状態から選び直し、その保存が終わる前に提出を押す
    state.slowFailPuts = true
    await page
      .getByRole('group', { name: new RegExp(QUESTIONS[3].body) })
      .getByRole('radio', { name: 'あてはまらない', exact: true })
      .check()
    await page.getByRole('button', { name: '提出する' }).click()

    await expect(page.getByText('保存できていない回答があります')).toBeVisible()
    expect(state.submits).toBe(0)
    await expect(page).toHaveURL(new RegExp(`/assessments/${SESSION_ID}$`))
  })

  test('読み込み中は枠を出し、検査が無ければ理由と次の行動を出す', async ({ page }) => {
    await mockBackend(page, { templates: 0, listDelayMs: 1500 })
    await page.goto('/assessments')
    await expect(page.getByRole('status', { name: '検査を読み込み中' })).toBeVisible({ timeout: 15000 })
    await expect(page.getByText('いま受けられる検査はありません')).toBeVisible()
    await expect(page.getByRole('button', { name: 'AI相談へ' })).toBeVisible()
  })

  test('他人の・存在しない受検はエラーと一覧への導線を出す', async ({ page }) => {
    await mockBackend(page)
    await page.goto('/assessments/11111111-1111-1111-1111-111111111111')
    await expect(page.getByText('この検査は見つかりませんでした')).toBeVisible({ timeout: 15000 })
    await page.getByRole('button', { name: '一覧へ戻る' }).click()
    await expect(page).toHaveURL(/\/assessments$/)
  })
})
