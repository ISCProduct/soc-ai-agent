import { test, expect } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

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
