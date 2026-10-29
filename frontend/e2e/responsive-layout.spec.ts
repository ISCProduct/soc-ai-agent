import { test, expect } from '@playwright/test'
import { setupAuth, TEST_ADMIN, TEST_USER } from './fixtures/auth'

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

// ES添削画面の列ずれ（#1783）。
// ヘッダーの帯は全幅の左寄せ、本文は maxWidth で中央寄せだったため、
// 広い画面ほど見出しと入力欄の左端がずれる。帯は全幅のまま、中身だけ本文と同じ列に入れる。
test.describe('レスポンシブ: ES添削画面の列揃え', () => {
  test('ヘッダーと本文の左端が揃い、戻るボタンは列の右端に揃う', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await stubApi(page)

    for (const width of [2000, 1440, 1024, 768]) {
      await page.setViewportSize({ width, height: 900 })
      await page.waitForFunction((w: number) => document.documentElement.clientWidth === w, width)
      await page.goto('/es-rewrite')
      await expect(page.getByText('ES添削・リライト', { exact: true })).toBeVisible({ timeout: 15000 })

      const headerRow = page.getByTestId('es-rewrite-header-row')
      const modeButton = page.getByRole('button', { name: 'ES添削' })
      const form = page.locator('.MuiPaper-root').filter({ hasText: '元の文章を入力' }).first()
      const backButton = page.getByRole('button', { name: 'ホームへ戻る' })
      const band = page.locator('header')

      await expect(headerRow).toBeVisible()
      await expect(form).toBeVisible()

      const headerRowBox = await headerRow.boundingBox()
      const modeButtonBox = await modeButton.boundingBox()
      const formBox = await form.boundingBox()
      const backButtonBox = await backButton.boundingBox()
      const bandBox = await band.boundingBox()
      expect(headerRowBox, `${width}px: ヘッダー行が取得できない`).not.toBeNull()
      expect(modeButtonBox, `${width}px: モード切替が取得できない`).not.toBeNull()
      expect(formBox, `${width}px: 入力欄が取得できない`).not.toBeNull()
      expect(backButtonBox, `${width}px: 戻るボタンが取得できない`).not.toBeNull()
      expect(bandBox, `${width}px: ヘッダー帯が取得できない`).not.toBeNull()

      expect(Math.abs(headerRowBox!.x - modeButtonBox!.x), `${width}px: 見出し列とモード切替の左端`).toBeLessThanOrEqual(1)
      expect(Math.abs(headerRowBox!.x - formBox!.x), `${width}px: 見出し列と入力欄の左端`).toBeLessThanOrEqual(1)
      expect(
        Math.abs(backButtonBox!.x + backButtonBox!.width - headerRowBox!.x - headerRowBox!.width),
        `${width}px: 戻るボタンが列の右端に揃っていない`,
      ).toBeLessThanOrEqual(1)

      const { scrollWidth, clientWidth } = await page.evaluate(() => ({
        scrollWidth: document.documentElement.scrollWidth,
        clientWidth: document.documentElement.clientWidth,
      }))
      expect(scrollWidth, `${width}px: 横方向にはみ出している`).toBeLessThanOrEqual(clientWidth)
      expect(Math.abs(bandBox!.width - clientWidth), `${width}px: ヘッダー帯が全幅ではない`).toBeLessThanOrEqual(1)
    }
  })
})

// 集合知画面の列ずれ（#1766）。
// 帯は管理シェルの AppBar で全幅。本文は maxWidth 720 の中央寄せだったため、
// 画面幅が 720 を超えてサイドバーが出る前（768px）だけ見出しが 32px 右へずれた。
// 帯が出ている幅ではテキストの左端を比べる。帯が消える md 以上は見出しが見えることだけ見る。
test.describe('レスポンシブ: 集合知画面の列揃え', () => {
  test('帯の見出しと本文見出しの左端が揃う', async ({ page }) => {
    await setupAuth(page, TEST_ADMIN)
    await stubApi(page)
    await page.route('**/api/admin/me/school-access*', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ restricted: false, schools: [] }),
      }),
    )

    for (const width of [2000, 1440, 1024, 768, 375]) {
      await page.setViewportSize({ width, height: 900 })
      await page.goto('/admin/collective-insights')

      const heading = page.getByRole('main').getByRole('heading', { name: '集合知サマリー管理' })
      await expect(heading).toBeVisible({ timeout: 15000 })
      const bandTitle = page.getByRole('banner').getByText('集合知サマリー再構築', { exact: true })

      // 帯（AppBar）は md（900px）未満だけ出る。出ていない幅では比べない。
      if (width < 900) {
        await expect(bandTitle, `${width}px: 帯の見出しが出ていない`).toBeVisible()
        const band = await bandTitle.boundingBox()
        const body = await heading.boundingBox()
        expect(band, `${width}px: 帯の見出しが取得できない`).not.toBeNull()
        expect(body, `${width}px: 本文見出しが取得できない`).not.toBeNull()
        expect(
          Math.abs(band!.x - body!.x),
          `${width}px で帯(x=${band!.x})と本文(x=${body!.x})の左端がずれている`,
        ).toBeLessThanOrEqual(8)
      }

      const { scrollWidth, clientWidth } = await page.evaluate(() => ({
        scrollWidth: document.documentElement.scrollWidth,
        clientWidth: document.documentElement.clientWidth,
      }))
      expect(scrollWidth, `${width}px: 横方向にはみ出している`).toBeLessThanOrEqual(clientWidth)
    }
  })
})
