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
