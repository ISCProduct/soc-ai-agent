import { test, expect } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'
import type { InterviewSession } from '../lib/interview'

// 下部ナビが画面外へはみ出さないことの回帰テスト（UI監査 2026-09-21 / R1）。
//
// document.scrollWidth が画面幅以内でも、ナビの子要素は画面外に出ていた。
// 親ではなく各項目の左右端を測る。
//
// 監査で実測した値:
//   320px: 診断が x=-40、設定の右端が 360（画面外で押せない）
//   375px: 診断が x=-12
//   390px: 診断が x=-5
//
// 監査は他に3件を指摘しているが、ここでは検証していない。
// 「通るが何も守らないテスト」は、テストが無いより危ないため置かない。
//
//   R2 予定表の列ずれ  カレンダーの再描画待ちが環境で安定しなかった。
//                      列幅は minmax(0,1fr) とセルの minWidth:0 で担保する。
//   R3 PCのヘッダー残り / のヘッダーがテスト環境では描画されず、
//                      修正を外しても差が出なかった。
//   R4 管理者の横並び  API をモックすると対象のフィルターが描画されない。
//                      実データのフィクスチャが要る（別途）。

const PHONE_WIDTHS = [390, 375, 320]

// CI ではバックエンドが起動していない。未モックの API を実サーバーへ流すと
// 応答を待ち続けて画面が「読み込み中」のまま止まる。空応答で即座に返す。
async function stubApi(page: import('@playwright/test').Page) {
  await page.route('**/api/**', (route) =>
    route.request().method() === 'GET'
      ? route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
      : route.fulfill({ status: 204, body: '' }),
  )
}

test.describe('レスポンシブ: 横方向の破綻', () => {
  test('学生の下部ナビが5項目とも画面内に収まる', async ({ page }) => {
    await stubApi(page)
    await setupAuth(page, TEST_USER)

    // 下部ナビは md 未満でだけ出る。先にモバイル幅にしてから開く。
    await page.setViewportSize({ width: 390, height: 768 })
    await page.goto('/schedule')

    const nav = page.locator('.MuiBottomNavigationAction-root')
    await nav.first().waitFor({ state: 'visible', timeout: 15000 })
    await expect(nav).toHaveCount(5)

    for (const width of PHONE_WIDTHS) {
      await page.setViewportSize({ width, height: 768 })
      // リサイズ後のレイアウト確定を待つ。
      await page.waitForFunction(
        (w: number) =>
          document.querySelectorAll('.MuiBottomNavigationAction-root').length === 5 &&
          Math.round(
            document.querySelector('.MuiBottomNavigation-root')!.getBoundingClientRect().width,
          ) === w,
        width,
        { timeout: 5000 },
      )

      // MUI既定の minWidth=80px は5項目で400px必要になり、
      // 400px未満では両端が画面外へ出る。
      const boxes = await page.evaluate(() =>
        [...document.querySelectorAll('.MuiBottomNavigationAction-root')].map((e) => {
          const r = e.getBoundingClientRect()
          return { label: (e as HTMLElement).innerText.trim(), x: r.x, right: r.right }
        }),
      )
      for (const b of boxes) {
        expect(b.x, `${width}px: ${b.label} の左端が画面内`).toBeGreaterThanOrEqual(-0.5)
        expect(b.right, `${width}px: ${b.label} の右端が画面内`).toBeLessThanOrEqual(width + 0.5)
      }
    }
  })
})

// 結果画面の列ずれの回帰テスト（本番で発覚 2026-10-15）。
//
// ヘッダーの帯は全幅のまま中身が左端に張り付き、本文だけ maxWidth:1200 で
// 中央寄せだったため、画面が広いほど見出しと一覧がずれた。実測値:
//   2000px: 見出し x=24 / 一覧 x=400（376px のずれ）
//   1440px: 見出し x=24 / 一覧 x=120（96px のずれ）
//   1024px: 見出し x=24 / 一覧 x=32（padding の差で 8px）
//
// 片方だけ直しても気付けないので、左端が一致することを測る。
test.describe('レスポンシブ: 結果画面の列揃え', () => {
  const RECOMMENDATIONS = {
    recommendations: [1, 2, 3].map((i) => ({
      id: i,
      category_name: `テスト株式会社${i}`,
      industry: 'IT',
      score: 90 - i,
      matched_axis_count: 3,
      reason: 'テスト用の推薦理由です。',
    })),
    evaluated_categories: 4,
    min_matched_axis_count: 3,
  }

  test('ヘッダーの見出しと企業一覧の左端が揃う', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await stubApi(page)
    // stubApi より後に登録したものが優先される
    await page.route('**/api/chat/recommendations*', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(RECOMMENDATIONS) }),
    )

    for (const width of [2000, 1440, 1024]) {
      await page.setViewportSize({ width, height: 900 })
      await page.goto('/results?user_id=1&session_id=layout-check')

      const heading = page.getByRole('heading', { level: 1 })
      await expect(heading).toBeVisible()
      const firstRow = page.locator('ol > li').first()
      await expect(firstRow).toBeVisible()

      const h = await heading.boundingBox()
      const r = await firstRow.boundingBox()
      expect(h, `${width}px: 見出しが取得できない`).not.toBeNull()
      expect(r, `${width}px: 企業一覧が取得できない`).not.toBeNull()
      // 1px はブラウザの丸め誤差の許容
      expect(
        Math.abs((h?.x ?? 0) - (r?.x ?? 0)),
        `${width}px で見出し(x=${h?.x})と一覧(x=${r?.x})の左端がずれている`,
      ).toBeLessThanOrEqual(1)
    }
  })
})

// AI面接・企業選択画面の列ずれの回帰テスト（SOCAIAGENT-443 / #1784）。
//
// /results（#1761）と同一原因: ヘッダーの帯の中身を直接 flex で全幅に
// 広げ、本文だけ maxWidth:896 で中央寄せしていたため、画面が広いほど
// 帯の行と本文の左端がずれた。実測値:
//   2000px: 帯 x=124 / 本文 x=552（428pxのずれ）
//   1440px: 帯 x=124 / 本文 x=272（148pxのずれ）
//   768px : 帯 x=68  / 本文 x=24（44pxのずれ。向きが逆転する幅なので必ず含める）
//
// 帯側はアイコン+ギャップの分だけ見出しテキストが右へ寄るため、テキスト同士
// ではなく「行全体」のボックス（data-testid="selection-header-row"）の
// 左端で本文の見出し(h1)と比べる。
test.describe('レスポンシブ: AI面接・企業選択画面の列揃え', () => {
  test('ヘッダーの行と本文見出しの左端が揃う', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await stubApi(page)

    for (const width of [2000, 1440, 1024, 768]) {
      await page.setViewportSize({ width, height: 900 })
      await page.goto('/interview')

      const headerRow = page.getByTestId('selection-header-row')
      await expect(headerRow).toBeVisible()
      const heading = page.getByRole('heading', { level: 1 })
      await expect(heading).toBeVisible()

      const row = await headerRow.boundingBox()
      const h = await heading.boundingBox()
      expect(row, `${width}px: ヘッダー行が取得できない`).not.toBeNull()
      expect(h, `${width}px: 見出しが取得できない`).not.toBeNull()
      // 1px はブラウザの丸め誤差の許容
      expect(
        Math.abs((row?.x ?? 0) - (h?.x ?? 0)),
        `${width}px でヘッダー行(x=${row?.x})と見出し(x=${h?.x})の左端がずれている`,
      ).toBeLessThanOrEqual(1)
    }
  })
})

// 面接履歴画面の列ずれ（SOCAIAGENT-444 / #1781）。
//
// ヘッダーの中身は全幅の左寄せ、本文は maxWidth:1100 で中央寄せのため、
// 広い画面ほどヘッダー見出しとトレンド・履歴列がずれる。
// Controller.List の応答形（sessions/total/page/limit）と
// Controller.GetTrend の応答形（points）に合わせてモックする。
test.describe('レスポンシブ: 面接履歴画面の列揃え', () => {
  // 修正後の実測値（ヘッダー行・トレンド・履歴一覧の左端）:
  // 2000px: 482 / 1440px: 202 / 1024px: 32 / 768px: 16
  const SESSION_RESPONSE: { sessions: InterviewSession[]; total: number; page: number; limit: number } = {
    sessions: [{
      id: 1,
      user_id: 1,
      status: 'finished',
      language: 'ja',
      started_at: '2026-10-01T01:00:00Z',
      ended_at: '2026-10-01T01:30:00Z',
      estimated_cost_usd: 0.123,
      template_version: 'v1',
      created_at: '2026-10-01T01:00:00Z',
      updated_at: '2026-10-01T01:30:00Z',
    }],
    total: 1,
    page: 1,
    limit: 10,
  }

  async function mockInterviewHistoryApi(
    page: import('@playwright/test').Page,
    sessions: typeof SESSION_RESPONSE.sessions,
    total: number,
  ) {
    // stubApi より後の route が優先される。
    await page.route('**/api/interviews**', (route) => {
      const { pathname } = new URL(route.request().url())
      if (pathname === '/api/interviews') {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ...SESSION_RESPONSE, sessions, total }),
        })
      }
      if (pathname === '/api/interviews/trend') {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ points: [] }),
        })
      }
      return route.fallback()
    })
  }

  async function measureAlignment(
    page: import('@playwright/test').Page,
    historyContent: 'session' | 'empty',
  ) {
    const headerRow = page.getByTestId('history-header-row')
    const trend = page.locator('.MuiPaper-root').filter({ hasText: 'パフォーマンストレンド' }).first()
    const backButton = page.getByRole('button', { name: '面接画面へ戻る' })
    const listContent = historyContent === 'session'
      ? page.locator('.MuiPaper-root').filter({
          has: page.getByText('面接セッション #1', { exact: true }),
        }).first()
      : page.locator('.MuiPaper-root').filter({ hasText: 'まだ面接の練習記録がありません' }).first()

    await expect(headerRow).toBeVisible()
    await expect(trend).toBeVisible()
    await expect(backButton).toBeVisible()
    await expect(listContent).toBeVisible()

    const headerRowBox = await headerRow.boundingBox()
    const trendBox = await trend.boundingBox()
    const backButtonBox = await backButton.boundingBox()
    const listBox = await listContent.boundingBox()
    expect(headerRowBox, 'ヘッダー行が取得できない').not.toBeNull()
    expect(trendBox, 'トレンドが取得できない').not.toBeNull()
    expect(backButtonBox, '戻るボタンが取得できない').not.toBeNull()
    expect(listBox, '履歴一覧または空状態が取得できない').not.toBeNull()

    return {
      width: page.viewportSize()!.width,
      headerRowX: headerRowBox!.x,
      trendX: trendBox!.x,
      listX: listBox!.x,
      trendDifference: Math.abs(headerRowBox!.x - trendBox!.x),
      listDifference: Math.abs(headerRowBox!.x - listBox!.x),
      backButtonRight: backButtonBox!.x + backButtonBox!.width,
      backButtonDifference: Math.abs(
        backButtonBox!.x + backButtonBox!.width - headerRowBox!.x - headerRowBox!.width,
      ),
    }
  }

  for (const historyContent of ['session', 'empty'] as const) {
    test(`${historyContent === 'session' ? '履歴あり' : '空状態'}でもヘッダー・本文の左端が揃う`, async ({ page }) => {
      await setupAuth(page, TEST_USER)
      await stubApi(page)
      await mockInterviewHistoryApi(
        page,
        historyContent === 'session' ? SESSION_RESPONSE.sessions : [],
        historyContent === 'session' ? 1 : 0,
      )

      const measurements: Array<Awaited<ReturnType<typeof measureAlignment>>> = []
      for (const width of [2000, 1440, 1024, 768]) {
        await page.setViewportSize({ width, height: 900 })
        await page.waitForFunction((w: number) => document.documentElement.clientWidth === w, width)
        await page.goto('/interview/history')
        await expect(
          historyContent === 'session'
            ? page.getByText('面接セッション #1', { exact: true })
            : page.getByText('まだ面接の練習記録がありません'),
        ).toBeVisible({ timeout: 15000 })
        const measurement = await measureAlignment(page, historyContent)
        const { scrollWidth, clientWidth } = await page.evaluate(() => ({
          scrollWidth: document.documentElement.scrollWidth,
          clientWidth: document.documentElement.clientWidth,
        }))
        expect(scrollWidth, `${width}px: 横方向にはみ出している`).toBeLessThanOrEqual(clientWidth)
        measurements.push(measurement)
      }

      const misaligned = measurements.filter(
        ({ trendDifference, listDifference, backButtonDifference }) =>
          trendDifference > 1 ||
          listDifference > 1 ||
          backButtonDifference > 1,
      )
      expect(
        misaligned,
        '1pxを超えてずれている幅（見出し・トレンド・履歴の左端、および戻るボタン・列の右端）',
      ).toEqual([])
    })
  }

  test('教員モードのチップが見出しと同じ行に収まり、戻るボタンも列の右端に揃う', async ({ page }) => {
    await setupAuth(page, { ...TEST_USER, role: 'teacher' })
    await stubApi(page)
    await mockInterviewHistoryApi(page, [], 0)

    for (const width of [2000, 1440, 1024, 768]) {
      await page.setViewportSize({ width, height: 900 })
      await page.waitForFunction((w: number) => document.documentElement.clientWidth === w, width)
      await page.goto('/interview/history')
      await expect(page.getByText('まだ面接の練習記録がありません')).toBeVisible({ timeout: 15000 })

      const headerRow = page.getByTestId('history-header-row')
      const heading = page.getByRole('heading', { name: '面接履歴', exact: true })
      const teacherChip = page.getByText('教員モード', { exact: true })
      const backButton = page.getByRole('button', { name: '面接画面へ戻る' })
      const trend = page.locator('.MuiPaper-root').filter({ hasText: 'パフォーマンストレンド' }).first()
      await expect(headerRow).toBeVisible()
      await expect(teacherChip).toBeVisible()

      const headerRowBox = await headerRow.boundingBox()
      const headingBox = await heading.boundingBox()
      const chipBox = await teacherChip.boundingBox()
      const backButtonBox = await backButton.boundingBox()
      const trendBox = await trend.boundingBox()
      expect(headerRowBox, `${width}px: ヘッダー行が取得できない`).not.toBeNull()
      expect(headingBox, `${width}px: 見出しが取得できない`).not.toBeNull()
      expect(chipBox, `${width}px: 教員モードのチップが取得できない`).not.toBeNull()
      expect(backButtonBox, `${width}px: 戻るボタンが取得できない`).not.toBeNull()
      expect(trendBox, `${width}px: トレンドが取得できない`).not.toBeNull()
      expect(
        Math.abs(headingBox!.y + headingBox!.height / 2 - (chipBox!.y + chipBox!.height / 2)),
        `${width}px: 教員モードのチップが見出しと同じ行にない`,
      ).toBeLessThanOrEqual(1)
      expect(
        Math.abs(backButtonBox!.x + backButtonBox!.width - headerRowBox!.x - headerRowBox!.width),
        `${width}px: 戻るボタンが列の右端に揃っていない`,
      ).toBeLessThanOrEqual(1)
      const { scrollWidth, clientWidth } = await page.evaluate(() => ({
        scrollWidth: document.documentElement.scrollWidth,
        clientWidth: document.documentElement.clientWidth,
      }))
      expect(scrollWidth, `${width}px: 横方向にはみ出している`).toBeLessThanOrEqual(clientWidth)
    }
  })
})
