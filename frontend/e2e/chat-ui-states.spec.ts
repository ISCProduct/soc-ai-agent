import { test, expect, type Page } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

function json(body: unknown) {
  return { status: 200, contentType: 'application/json', body: JSON.stringify(body) }
}

async function silenceHome(page: Page) {
  for (const path of ['**/api/schedule', '**/api/whats-new']) {
    await page.route(path, (route) => route.fulfill(json([])))
  }
  await page.route('**/api/applications', (route) => route.fulfill(json({ applications: [] })))
  await page.route('**/api/user/guidances', (route) => route.fulfill(json({ guidances: [] })))
  await page.route('**/api/resume/status', (route) =>
    route.fulfill(json({ has_document: true, latest_score: 90, needs_attention: false })),
  )
}

const log = (page: Page) => page.getByRole('log', { name: '自己分析チャットのやり取り' })

// 診断を始める前のホーム（相談中心）。履歴が0件のときは、チャットではなくこちらが出る。
const consultHeading = (page: Page) => page.getByRole('heading', { level: 1, name: /こんにちは/ })
const consultFreeInput = (page: Page) => page.getByRole('textbox', { name: /自分の言葉で入力する/ })
/** 初回の相談画面から自由入力で1通送り、チャット表示へ切り替える。 */
async function sendFirstAnswer(page: Page, text: string) {
  await expect(consultHeading(page)).toBeVisible({ timeout: 20000 })
  await consultFreeInput(page).fill(text)
  await page.getByRole('button', { name: '送る' }).click()
}

// #1615: チャットの状態表示とアクセシビリティ。実ログイン不要のモック認証で CI でも回す。
test.describe('チャットの状態表示', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await silenceHome(page)
  })

  test('履歴の読み込み中は骨格と読み上げ用の状態を出し、届いたら挨拶と次の操作に替わる', async ({ page }) => {
    let release: () => void = () => {}
    const gate = new Promise<void>((r) => { release = r })
    await page.route('**/api/chat/history*', async (route) => {
      await gate
      await route.fulfill(json([]))
    })

    await page.goto('/')
    await expect(page.getByRole('status').filter({ hasText: 'チャット履歴を読み込んでいます' })).toBeAttached({ timeout: 20000 })
    await expect(log(page).locator('.MuiSkeleton-root').first()).toBeVisible()

    release()
    // 届いたら、初回（履歴0件）は挨拶と、押して答えられる4つの選択肢を出す（骨格は消える）
    await expect(consultHeading(page)).toBeVisible()
    await expect(page.locator('.MuiSkeleton-root')).toHaveCount(0)
    const options = page.getByRole('group', { name: 'はじめに、いまの気持ちに近いものを選んでください。' })
    await expect(options.getByRole('button')).toHaveCount(4)
    await expect(page.getByRole('region', { name: '診断の進み具合' }).getByText('判定の進み具合 0%')).toBeVisible()
  })

  test('履歴の取得に失敗したら空のままにせず、理由と再試行を出す', async ({ page }) => {
    let fail = true
    await page.route('**/api/chat/history*', (route) =>
      fail ? route.fulfill({ status: 500, contentType: 'application/json', body: '{}' }) : route.fulfill(json([])),
    )

    await page.goto('/')
    await expect(log(page)).toContainText('履歴の読み込みに失敗しました', { timeout: 20000 })
    fail = false
    await log(page).getByRole('button', { name: '再試行' }).click()
    // 取り直せたら、エラーは消え、初回の相談画面になる
    await expect(consultHeading(page)).toBeVisible()
    await expect(page.getByText('履歴の読み込みに失敗しました')).toHaveCount(0)
  })

  test('AIの返答は発言者付きの区切りとして読み上げ領域へ追加される', async ({ page }) => {
    await page.route('**/api/chat/history*', (route) => route.fulfill(json([])))
    await page.route('**/api/chat', (route) =>
      route.fulfill(json({ response: 'ありがとうございます。次の質問です。', answered_questions: 1, total_questions: 15 })),
    )

    await page.goto('/')
    // 初回の相談画面から1通送ると、チャット表示（案内・自分の回答・AIの返答の3件）に切り替わる
    await sendFirstAnswer(page, 'ITエンジニア')
    await expect(log(page)).toHaveAttribute('aria-live', 'polite')
    await expect(log(page)).toHaveAttribute('aria-relevant', 'additions')

    const articles = log(page).getByRole('article')
    await expect(articles).toHaveCount(3, { timeout: 20000 })
    await expect(articles.nth(2)).toContainText('次の質問です')
    await expect(articles.nth(2)).toHaveAttribute('aria-label', /AI|アシスタント|エージェント/)
  })
})

test.describe('動きを減らす設定', () => {
  test('入力中表示と骨格のアニメーションを止める', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await setupAuth(page, TEST_USER)
    await silenceHome(page)
    let release: () => void = () => {}
    const gate = new Promise<void>((r) => { release = r })
    await page.route('**/api/chat/history*', async (route) => {
      await gate
      await route.fulfill(json([]))
    })
    let answer: () => void = () => {}
    const answered = new Promise<void>((r) => { answer = r })
    await page.route('**/api/chat', async (route) => {
      await answered
      await route.fulfill(json({ response: '次の質問です。', answered_questions: 1, total_questions: 15 }))
    })

    await page.goto('/')
    const skeleton = log(page).locator('.MuiSkeleton-root').first()
    await expect(skeleton).toBeVisible({ timeout: 20000 })
    expect(await skeleton.evaluate((el) => getComputedStyle(el).animationName)).toBe('none')
    release()

    await sendFirstAnswer(page, 'ITエンジニア')

    const typing = page.getByRole('status', { name: 'エージェントが入力中です' })
    await expect(typing).toBeVisible()
    const spinner = typing.locator('.MuiCircularProgress-root')
    expect(await spinner.evaluate((el) => getComputedStyle(el).animationName)).toBe('none')
    answer()
    await expect(typing).toHaveCount(0)
  })
})
