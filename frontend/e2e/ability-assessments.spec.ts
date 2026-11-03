import { test, expect, type Page, type Route } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

const SESSION_ID = '7c1f2a44-8b61-4f0e-9d22-5a3b6c7d8e90'
const QUESTIONS = [
  { id: 201, position: 1, section: 'verbal', body: '「審議」の意味として最も適切なものはどれか。', choices: ['物事をよく調べて話し合うこと', '決まったことを通知すること', '上位者の判断を仰ぐこと', '前例に従って処理すること'] },
  { id: 202, position: 2, section: 'numerical', body: '原価800円の商品に25%の利益を見込んだ売価はいくらか。', choices: ['825円', '1000円', '1025円', '1200円'] },
]

/** 正解の添字。サーバ側だけが持つ情報なので、モックの中にだけ置く。 */
const CORRECT: Record<number, number> = { 201: 0, 202: 1 }

type MockOptions = { remainingSeconds?: number; gone?: boolean }

/**
 * Backend の代わりに受検状態を持つ。
 * 締め切りの判定はサーバ側なので、ここで 410 を返す側として振る舞う。
 */
async function mockBackend(page: Page, opts: MockOptions = {}) {
  const state = {
    status: 'not_started' as 'not_started' | 'in_progress' | 'submitted',
    answers: {} as Record<string, number>,
    puts: [] as { id: string; choice: number }[],
    submits: 0,
    gone: opts.gone ?? false,
  }
  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
  const view = () => ({
    session_id: SESSION_ID,
    title: '能力検査（言語・非言語）',
    status: state.status,
    questions: QUESTIONS,
    answers: state.answers,
    remaining_seconds: opts.remainingSeconds ?? 900,
  })

  await page.route('**/api/ability-assessments**', async (route) => {
    const req = route.request()
    const path = new URL(req.url()).pathname.replace(/^\/api\/ability-assessments/, '')
    if (req.method() === 'GET' && path === '') {
      return json(route, {
        assessments: [{
          template_id: 1,
          title: '能力検査（言語・非言語）',
          description: '言語・非言語の基礎的な問題です',
          question_count: QUESTIONS.length,
          time_limit_seconds: 900,
          status: state.gone && state.status === 'in_progress' ? 'expired' : state.status,
          session_id: state.status === 'not_started' ? undefined : SESSION_ID,
          answered_count: Object.keys(state.answers).length,
        }],
      })
    }
    if (req.method() === 'POST' && path === '/1/sessions') {
      if (state.gone) return json(route, { error: '制限時間を過ぎました' }, 410)
      if (state.status === 'not_started') state.status = 'in_progress'
      return json(route, view())
    }
    if (req.method() === 'GET' && path === `/sessions/${SESSION_ID}`) {
      if (state.gone && state.status === 'in_progress') return json(route, { error: '制限時間を過ぎました' }, 410)
      return json(route, view())
    }
    const put = path.match(new RegExp(`^/sessions/${SESSION_ID}/answers/(\\d+)$`))
    if (req.method() === 'PUT' && put) {
      if (state.gone) return json(route, { error: '制限時間を過ぎました' }, 410)
      const { choice } = req.postDataJSON() as { choice: number }
      state.answers[put[1]] = choice
      state.puts.push({ id: put[1], choice })
      return route.fulfill({ status: 204 })
    }
    if (req.method() === 'POST' && path === `/sessions/${SESSION_ID}/submit`) {
      state.submits++
      if (state.status === 'submitted') return json(route, { error: 'この検査はすでに提出済みです' }, 409)
      // 締め切り後でも提出は通る（サーバの仕様）
      state.status = 'submitted'
      return json(route, { session_id: SESSION_ID })
    }
    if (req.method() === 'GET' && path === `/sessions/${SESSION_ID}/result`) {
      // 保存済みの回答から数える。固定値を返すと、正解を選んでも誤った表示を
      // 検証することになり、採点の回帰を検出できない。
      const sections: Record<string, { correct: number; total: number }> = {}
      for (const q of QUESTIONS) {
        const s = (sections[q.section] ??= { correct: 0, total: 0 })
        s.total++
        if (state.answers[String(q.id)] === CORRECT[q.id]) s.correct++
      }
      return json(route, {
        session_id: SESSION_ID,
        title: '能力検査（言語・非言語）',
        sections,
        order: ['verbal', 'numerical'],
      })
    }
    return json(route, { error: 'not mocked' }, 404)
  })
  return state
}

async function choose(page: Page, questionBody: string, label: string) {
  const group = page.getByRole('group', { name: new RegExp(questionBody.slice(0, 12)) })
  await group.getByRole('radio', { name: label, exact: true }).check()
  await expect(group.getByText('✓ 保存しました')).toBeVisible()
}

// SOCAIAGENT-433: 能力検査の受検（制限時間あり）
test.describe('能力検査', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)
  })

  test('制限時間と残り時間が出て、回答して提出すると分野別の正答数が出る', async ({ page }) => {
    const state = await mockBackend(page)

    await page.goto('/ability-assessments')
    await expect(page.getByRole('heading', { name: '能力検査', level: 1 })).toBeVisible()
    // 一覧に制限時間が出る（始める前に時間の見当がつく）
    await expect(page.getByTestId('ability-meta')).toContainText('制限時間 15分')
    await expect(page.getByTestId('ability-status')).toContainText('未受検')

    await page.getByRole('button', { name: '受検を始める' }).click()
    await expect(page).toHaveURL(new RegExp(`/ability-assessments/${SESSION_ID}$`))

    // 残り時間が見える
    await expect(page.getByTestId('ability-remaining')).toContainText('14:5')

    // 1問目（言語）。分野のラベルも出る
    await expect(page.getByText('言語', { exact: true })).toBeVisible()
    await choose(page, '「審議」の意味', '物事をよく調べて話し合うこと')
    await page.getByRole('button', { name: '次へ' }).click()

    // 2問目（非言語）
    await expect(page.getByText('非言語', { exact: true })).toBeVisible()
    await choose(page, '原価800円', '1000円')

    await page.getByRole('button', { name: '提出する' }).click()
    await expect(page).toHaveURL(new RegExp(`/ability-assessments/${SESSION_ID}/result$`))

    // 分野別の正答数。偏差値・順位は出さない
    // どちらも正解を選んだので 1/1。固定値ではなく保存済みの回答から数えている
    await expect(page.getByTestId('ability-section-verbal')).toContainText('1 / 1 問')
    await expect(page.getByTestId('ability-section-numerical')).toContainText('1 / 1 問')
    await expect(page.getByRole('heading', { name: 'この結果の使われ方' })).toBeVisible()
    await expect(page.getByText(/おすすめ企業の計算には使いません/)).toBeVisible()
    await expect(page.locator('body')).not.toContainText('偏差値')
    expect(state.submits).toBe(1)
  })

  test('途中でやめても回答が残り、再開して続きから答えられる', async ({ page }) => {
    const state = await mockBackend(page)

    await page.goto('/ability-assessments')
    await page.getByRole('button', { name: '受検を始める' }).click()
    await choose(page, '「審議」の意味', '物事をよく調べて話し合うこと')

    await page.getByRole('button', { name: '中断して一覧へ戻る' }).click()
    await expect(page.getByTestId('ability-status')).toContainText('受検中')
    await expect(page.getByTestId('ability-meta')).toContainText('2問中 1問 回答済み')

    await page.getByRole('button', { name: '続きから再開する' }).click()
    // 最初の未回答（2問目）から始まる
    await expect(page.getByText('原価800円の商品に25%の利益を見込んだ売価はいくらか。')).toBeVisible()
    expect(state.answers['201']).toBe(0)
  })

  test('わからない問題を飛ばしても提出でき、未回答は誤答として数えると伝える', async ({ page }) => {
    await mockBackend(page)

    await page.goto('/ability-assessments')
    await page.getByRole('button', { name: '受検を始める' }).click()
    await expect(page.getByText(/未回答は誤答として数えます/)).toBeVisible()

    // 1問も答えずに最後まで進めて提出する
    await page.getByRole('button', { name: '次へ' }).click()
    await page.getByRole('button', { name: '提出する' }).click()
    await expect(page).toHaveURL(new RegExp(`/ability-assessments/${SESSION_ID}/result$`))
  })

  test('制限時間を過ぎたら回答できず、そこまでの回答で提出できる', async ({ page }) => {
    const state = await mockBackend(page)

    await page.goto('/ability-assessments')
    await page.getByRole('button', { name: '受検を始める' }).click()
    await choose(page, '「審議」の意味', '物事をよく調べて話し合うこと')

    // ここでサーバ側の締め切りが過ぎる
    state.gone = true

    // 選び直すと 410 が返り、「保存できませんでした」ではなく時間切れとして伝える
    const group = page.getByRole('group', { name: /「審議」の意味/ })
    await group.getByRole('radio', { name: '決まったことを通知すること', exact: true }).check()
    await expect(page.getByTestId('ability-time-up')).toBeVisible()
    await expect(page.getByText(/制限時間を過ぎたため、この回答は記録されていません/)).toBeVisible()
    await expect(page.getByText(/もう一度選んでください/)).toHaveCount(0)

    // 時間切れでも提出はできる
    await page.getByRole('button', { name: '提出する' }).click()
    await expect(page).toHaveURL(new RegExp(`/ability-assessments/${SESSION_ID}/result$`))
    expect(state.submits).toBe(1)
  })

  test('時間切れ後に開き直すと、提出だけを促す', async ({ page }) => {
    const state = await mockBackend(page)

    await page.goto('/ability-assessments')
    await page.getByRole('button', { name: '受検を始める' }).click()
    await choose(page, '「審議」の意味', '物事をよく調べて話し合うこと')
    state.gone = true

    await page.goto(`/ability-assessments/${SESSION_ID}`)
    await expect(page.getByTestId('ability-time-up')).toBeVisible()
    // 問題は出さない（答えられないのに出すと操作できると誤解させる）
    await expect(page.getByText('「審議」の意味として最も適切なものはどれか。')).toHaveCount(0)
    await expect(page.getByRole('button', { name: '提出する' })).toBeEnabled()
  })

  test('他人の・存在しない受検はエラーと一覧への導線を出す', async ({ page }) => {
    await page.route('**/api/ability-assessments**', (route) =>
      route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: '検査が見つかりません' }) }),
    )
    await page.goto(`/ability-assessments/${SESSION_ID}`)
    await expect(page.getByText(/この検査は見つかりませんでした/)).toBeVisible()
    await expect(page.getByRole('button', { name: '一覧へ戻る' })).toBeVisible()
  })
})
