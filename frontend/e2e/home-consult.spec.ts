import { test, expect, type Page } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

function json(body: unknown) {
  return { status: 200, contentType: 'application/json', body: JSON.stringify(body) }
}

type HomeMock = { guidances?: unknown[]; schedule?: unknown[] }

// SOCAIAGENT-668: ホームを「相談中心」の3カラムにする（Figma 改善G）。
async function mockHome(page: Page, opts: HomeMock = {}) {
  await page.route('**/api/chat/history*', (route) => route.fulfill(json([])))
  await page.route('**/api/schedule', (route) => route.fulfill(json(opts.schedule ?? [])))
  await page.route('**/api/applications', (route) => route.fulfill(json({ applications: [] })))
  await page.route('**/api/bff/user/guidances', (route) => route.fulfill(json({ guidances: opts.guidances ?? [] })))
  await page.route('**/api/whats-new', (route) => route.fulfill(json([])))
  await page.route('**/api/resume/status', (route) =>
    route.fulfill(json({ has_document: true, latest_score: 90, needs_attention: false })),
  )
  await page.route('**/api/notifications*', (route) => route.fulfill(json({ notifications: [], unread_count: 0 })))
}

const OPTION = '就活の進め方がわからない'
const heading = (page: Page) => page.getByRole('heading', { level: 1, name: /こんにちは/ })
const rightPanel = (page: Page) => page.getByRole('complementary', { name: '先生からの案内・予定・お知らせ' })

test.describe('ホーム（相談中心・広い画面）', () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    await setupAuth(page, TEST_USER)
  })

  test('挨拶・4択・自由入力・進捗帯と、左サイドバー・右パネルの3カラムを出す', async ({ page }) => {
    await mockHome(page)
    await page.goto('/')

    await expect(heading(page)).toBeVisible({ timeout: 20000 })
    await expect(page.getByText('進路について、気軽に相談できます。')).toBeVisible()
    const options = page.getByRole('group', { name: 'はじめに、いまの気持ちに近いものを選んでください。' })
    await expect(options.getByRole('button')).toHaveCount(4)
    await expect(page.getByRole('textbox', { name: /自分の言葉で入力する/ })).toBeVisible()
    await expect(page.getByText('入力した内容にもとづいて、次の質問が変わります。')).toBeVisible()
    await expect(page.getByRole('region', { name: '診断の進み具合' }).getByText('判定の進み具合 0%')).toBeVisible()

    // 右パネルの3つの欄
    const panel = rightPanel(page)
    await expect(panel.getByRole('heading', { name: '先生から' })).toBeVisible()
    await expect(panel.getByRole('heading', { name: 'これからの予定' })).toBeVisible()
    await expect(panel.getByRole('heading', { name: 'お知らせ' })).toBeVisible()
    await expect(panel.getByText('予定はまだありません。')).toBeVisible()

    // 左サイドバー: 現在地はホーム。すべての項目にアイコンがある
    const nav = page.getByRole('navigation', { name: 'メニュー' }).last()
    await expect(nav.getByRole('link', { name: 'ホーム' })).toHaveAttribute('aria-current', 'page')
    const links = nav.getByRole('link')
    const count = await links.count()
    expect(count).toBeGreaterThanOrEqual(14)
    for (let i = 0; i < count; i++) {
      await expect(links.nth(i).locator('svg').first()).toBeAttached()
    }
  })

  test('選択肢を押すと、その文言が回答として送られ、チャット表示に切り替わる', async ({ page }) => {
    await mockHome(page)
    const posted: string[] = []
    const intents: Array<string | undefined> = []
    await page.route('**/api/chat', async (route) => {
      const body = JSON.parse(route.request().postData() ?? '{}')
      posted.push(body.message ?? '')
      intents.push(body.consult_intent)
      await route.fulfill(
        json({ response: 'ありがとうございます。次の質問です。', answered_questions: 1, total_questions: 15, judgment_percent: 18 }),
      )
    })

    await page.goto('/')
    await expect(heading(page)).toBeVisible({ timeout: 20000 })
    await page.getByRole('button', { name: OPTION }).click()

    await expect.poll(() => posted, { timeout: 10000 }).toEqual([OPTION])
    // 文言とあわせて、相談の意図（バックエンドへ保存される）も送る
    expect(intents).toEqual(['how_to_proceed'])
    // 診断が始まったので、チャット表示（自分の回答とAIの返答）になる。右パネルは残る
    const log = page.getByRole('log', { name: '自己分析チャットのやり取り' })
    await expect(log).toContainText(OPTION)
    await expect(log).toContainText('次の質問です')
    await expect(heading(page)).toHaveCount(0)
    await expect(rightPanel(page)).toBeVisible()
    // 判定の進み具合は、サーバーが返した値（18%）になる。サイドバーとチャットの上部の両方
    await expect(page.getByText('判定の進み具合 18%').first()).toBeVisible()
    await expect(page.getByRole('progressbar', { name: '自己分析の進み具合' }).first()).toHaveAttribute('aria-valuenow', '18')
  })

  test('自由入力は、入力した文章を送り、空のときは送れない', async ({ page }) => {
    await mockHome(page)
    const posted: string[] = []
    await page.route('**/api/chat', async (route) => {
      posted.push(JSON.parse(route.request().postData() ?? '{}').message ?? '')
      await route.fulfill(json({ response: '受け取りました', answered_questions: 1, total_questions: 15 }))
    })

    await page.goto('/')
    await expect(heading(page)).toBeVisible({ timeout: 20000 })
    await expect(page.getByRole('button', { name: '送る' })).toBeDisabled()
    await page.getByRole('textbox', { name: /自分の言葉で入力する/ }).fill('情報系の仕事に就きたい')
    await page.getByRole('button', { name: '送る' }).click()
    await expect.poll(() => posted, { timeout: 10000 }).toEqual(['情報系の仕事に就きたい'])
  })

  test('先生からの案内は右パネルに出て、「読みました」で既読にして消える', async ({ page }) => {
    await mockHome(page, {
      guidances: [{ id: 7, kind: 'low_match', message: '応募先を一緒に見直しましょう', created_at: '2026-10-01T00:00:00Z' }],
    })
    let dismissed = 0
    await page.route('**/api/bff/user/guidances/7/dismiss', async (route) => {
      dismissed++
      await route.fulfill(json({}))
    })

    await page.goto('/')
    const panel = rightPanel(page)
    await expect(panel.getByText('応募先を一緒に見直しましょう')).toBeVisible({ timeout: 20000 })
    await panel.getByRole('button', { name: '読みました' }).click()
    await expect(panel.getByText('応募先を一緒に見直しましょう')).toHaveCount(0)
    await expect(panel.getByText('先生からの案内はありません。')).toBeVisible()
    expect(dismissed).toBe(1)
  })

  test('案内の取得に失敗したら、その欄だけにエラーと再読み込みを出し、ほかの欄とメインは使える', async ({ page }) => {
    await mockHome(page)
    let fail = true
    await page.route('**/api/bff/user/guidances', (route) =>
      fail
        ? route.fulfill({ status: 500, contentType: 'application/json', body: '{}' })
        : route.fulfill(json({ guidances: [{ id: 1, kind: 'low_match', message: '応募先を一緒に見直しましょう', created_at: '2026-10-01T00:00:00Z' }] })),
    )

    await page.goto('/')
    const panel = rightPanel(page)
    await expect(panel.getByText('先生からの案内を読み込めませんでした')).toBeVisible({ timeout: 20000 })
    // 案内が失敗しても、予定の欄とメインの相談は使える
    await expect(panel.getByText('予定はまだありません。')).toBeVisible()
    await expect(page.getByRole('button', { name: OPTION })).toBeEnabled()

    fail = false
    await panel.getByRole('button', { name: '再読み込み' }).first().click()
    await expect(panel.getByText('応募先を一緒に見直しましょう')).toBeVisible()
    await expect(panel.getByText('先生からの案内を読み込めませんでした')).toHaveCount(0)
  })

  test('直近の予定は右パネルに出る', async ({ page }) => {
    const soon = new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString()
    await mockHome(page, {
      schedule: [{ company_name: '株式会社テスト', stage: 'interview', title: '一次面接', scheduled_at: soon }],
    })
    await page.goto('/')
    await expect(rightPanel(page).getByRole('link', { name: /株式会社テスト 一次面接/ })).toBeVisible({ timeout: 20000 })
    await expect(rightPanel(page).getByRole('link', { name: '予定を追加する' })).toHaveAttribute('href', '/schedule')
  })
})

test.describe('ホーム（相談中心・狭い画面）', () => {
  for (const appearance of [
    { mode: 'comfortable', ink: 'rgb(15, 23, 42)', icon: 'rgb(31, 95, 139)', mark: 'rgb(0, 114, 178)', radius: '12px', focus: '2px' },
    { mode: 'high-contrast', ink: 'rgb(0, 0, 0)', icon: 'rgb(11, 61, 145)', mark: 'rgb(11, 61, 145)', radius: '4px', focus: '3px' },
  ]) {
    test(`${appearance.mode}: メニューの色の役割・角丸・キーボードフォーカスが共通基準に従う`, async ({ page }) => {
      await setupAuth(page, TEST_USER)
      await mockHome(page)
      await page.addInitScript((mode) => window.localStorage.setItem('student-theme-mode', mode), appearance.mode)

      for (const width of [375, 1440]) {
        await page.setViewportSize({ width, height: 900 })
        await page.goto('/')
        await expect(heading(page)).toBeVisible({ timeout: 20000 })
        if (width < 900) await page.getByRole('button', { name: 'メニューを開く' }).click()

        const drawer = page.locator('.MuiDrawer-paper:visible')
        const menu = drawer.getByRole('navigation', { name: 'メニュー' })
        const home = menu.getByRole('link', { name: 'ホーム', exact: true })
        const results = menu.getByRole('link', { name: 'マッチング結果', exact: true })
        await expect(results).toHaveCSS('color', appearance.ink)
        await expect(results.locator('svg')).toHaveCSS('color', appearance.icon)
        await expect(home).toHaveCSS('color', appearance.mark)
        await expect(home.locator('svg')).toHaveCSS('color', appearance.mark)
        await expect(results).toHaveCSS('font-size', '16px')
        await expect(results).toHaveCSS('line-height', '25.6px')
        await expect(results).toHaveCSS('border-radius', appearance.radius)
        await expect(drawer).toHaveCSS('box-shadow', 'none')

        await home.focus()
        await page.keyboard.press('Tab')
        await expect(results).toBeFocused()
        await expect(results).toHaveCSS('outline-width', appearance.focus)
        await expect(results).toHaveCSS('outline-color', appearance.mark)
      }
    })
  }

  test('モバイルも基準の16pxの文字・44pxの行を使い、末尾までスクロールしても閉じられる', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await mockHome(page)

    for (const width of [320, 375, 768]) {
      await page.setViewportSize({ width, height: 640 })
      await page.goto('/')
      await expect(heading(page)).toBeVisible({ timeout: 20000 })
      await page.getByRole('button', { name: 'メニューを開く' }).click()

      const drawer = page.locator('.MuiDrawer-paper:visible')
      const close = drawer.getByRole('button', { name: 'メニューを閉じる' })
      const menu = drawer.getByRole('navigation', { name: 'メニュー' })
      const home = menu.getByRole('link', { name: 'ホーム', exact: true })
      await expect(drawer).toHaveCSS('width', `${Math.min(360, width - 24)}px`)
      await expect(home).toHaveCSS('font-size', '16px')
      await expect(home).toHaveCSS('min-height', '44px')
      await expect(menu).toHaveCSS('overflow-y', 'auto')
      await expect(menu).toHaveCSS('scrollbar-width', 'none')
      await expect(close).toHaveCSS('width', '44px')
      await expect(close).toHaveCSS('height', '44px')

      const before = await close.boundingBox()
      expect(before).not.toBeNull()
      await menu.evaluate((el) => { el.scrollTop = el.scrollHeight })
      await expect(menu.getByRole('link', { name: 'プロフィール設定' })).toBeInViewport()
      expect(await menu.evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
      const after = await close.boundingBox()
      expect(after).not.toBeNull()
      expect(Math.abs(after!.y - before!.y)).toBeLessThan(1)

      await close.click()
      await expect(close).not.toBeVisible()
    }
  })

  test('PCは280px幅・44pxの行を保ち、基準の16pxの文字を使う', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await mockHome(page)

    for (const width of [1024, 1440]) {
      await page.setViewportSize({ width, height: 900 })
      await page.goto('/')
      await expect(heading(page)).toBeVisible({ timeout: 20000 })
      const drawer = page.locator('.MuiDrawer-paper:visible')
      const home = drawer.getByRole('link', { name: 'ホーム', exact: true })
      await expect(drawer).toHaveCSS('width', '280px')
      await expect(home).toHaveCSS('font-size', '16px')
      await expect(home).toHaveCSS('min-height', '44px')
      await expect(drawer.getByRole('button', { name: 'メニューを閉じる' })).toHaveCount(0)
    }
  })

  test('右パネルは出さず、従来どおり上に「次にやること」を積む。メインの相談は同じ', async ({ page }) => {
    await page.setViewportSize({ width: 1100, height: 800 })
    await setupAuth(page, TEST_USER)
    await mockHome(page)
    await page.goto('/')

    await expect(heading(page)).toBeVisible({ timeout: 20000 })
    await expect(rightPanel(page)).toHaveCount(0)
    await expect(page.getByRole('region', { name: '次にやること' })).toBeVisible()
    await expect(page.getByRole('button', { name: OPTION })).toBeVisible()
  })
})
