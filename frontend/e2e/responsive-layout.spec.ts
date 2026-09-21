import { test, expect } from '@playwright/test'
import { setupAuth, TEST_USER, TEST_ADMIN } from './fixtures/auth'

// 画面が横に破綻しないことの回帰テスト（UI監査 2026-09-21）。
//
// 「document.scrollWidth が画面幅以内」だけでは足りない。下部ナビや
// カレンダーの子要素は、親が収まっていても画面外へ出ていた。
// 要素の左右端と、曜日行と日付行の列位置まで測る。
//
// 監査で実測した値:
//   320px: 下部ナビの診断が x=-40、設定の右端が 360（画面外）
//   390px: 管理者ダッシュボードの scrollWidth=468
//   予定表: 長い企業名の入った週だけ列幅が変わる

const PHONE_WIDTHS = [390, 375, 320]

// 計測対象の要素だけを見たいので、開発サーバーのオーバーレイが
// 前面に出ないよう待ってから測る。
async function settle(page: import('@playwright/test').Page) {
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(300)
}

test.describe('レスポンシブ: 横方向の破綻', () => {
  test('学生の下部ナビが5項目とも画面内に収まる', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    // ナビはどの学生画面にも出る。/ はチャットの初期化待ちが入るため、
    // 描画が安定している /schedule で測る。
    await page.goto('/schedule')
    await settle(page)

    for (const width of PHONE_WIDTHS) {
      await page.setViewportSize({ width, height: 768 })
      await page.waitForTimeout(200)

      const boxes = await page
        .locator('.MuiBottomNavigationAction-root')
        .evaluateAll((els) =>
          els.map((e) => {
            const r = e.getBoundingClientRect()
            return { label: (e as HTMLElement).innerText.trim(), x: r.x, right: r.right }
          }),
        )

      expect(boxes.length, `${width}px でナビ項目が5つ`).toBe(5)
      for (const b of boxes) {
        // MUI既定の minWidth=80px は5項目で400px必要になり、
        // 400px未満では両端が画面外へ出る。
        expect(b.x, `${width}px: ${b.label} の左端が画面内`).toBeGreaterThanOrEqual(-0.5)
        expect(b.right, `${width}px: ${b.label} の右端が画面内`).toBeLessThanOrEqual(width + 0.5)
      }
    }
  })

  test('PCではモバイル用ヘッダーが表示されない', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    await page.goto('/')
    await settle(page)

    // md(900px)以上ではチャット内のタイトルと重複するため出してはいけない。
    // CSS Modules の display:none が MUI の display:flex に負けていた。
    for (const width of [900, 1440, 1920]) {
      await page.setViewportSize({ width, height: 900 })
      await page.waitForTimeout(200)

      const visible = await page.locator('header').evaluateAll((els) =>
        els.filter((e) => {
          const s = getComputedStyle(e)
          return s.display !== 'none' && e.getBoundingClientRect().height > 0
        }).length,
      )
      expect(visible, `${width}px で表示中のheaderが無い`).toBe(0)
    }
  })

  test('予定表の曜日行と日付行の列幅が揃う', async ({ page }) => {
    await setupAuth(page, TEST_USER)
    // 月の中旬に置く。月末だと週の並びで再現しないことがある。
    const mid = new Date()
    mid.setDate(15)
    await page.route('**/api/schedule*', async (route) => {
      if (route.request().method() !== 'GET') return route.continue()
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 1,
            user_id: 1,
            // 長い企業名を入れた週だけ列幅が変わるのが元の不具合。
            company_name: '株式会社テクノロジーソリューションズ',
            title: '技術面接（バックエンド）',
            stage: '一次面接',
            scheduled_at: mid.toISOString(),
            notes: '',
            created_at: mid.toISOString(),
            updated_at: mid.toISOString(),
          },
        ]),
      })
    })
    await page.goto('/schedule')
    await settle(page)

    for (const width of [1920, 768, 390, 320]) {
      await page.setViewportSize({ width, height: 844 })
      await page.waitForTimeout(250)

      const grids = await page.evaluate(() =>
        [...document.querySelectorAll('div')]
          .filter((e) => {
            const s = getComputedStyle(e)
            return s.display === 'grid' && s.gridTemplateColumns.split(' ').length === 7
          })
          .map((g) =>
            getComputedStyle(g)
              .gridTemplateColumns.split(' ')
              .map((v) => Math.round(parseFloat(v))),
          ),
      )

      expect(grids.length, `${width}px で7列gridが2つ以上（曜日行＋週）`).toBeGreaterThanOrEqual(2)
      const head = grids[0].join(',')
      for (const row of grids.slice(1)) {
        // ずれると日付が別の曜日の下に並ぶ。
        expect(row.join(','), `${width}px で列幅が曜日行と一致`).toBe(head)
      }

      const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth)
      expect(scrollWidth, `${width}px で横スクロールしない`).toBeLessThanOrEqual(width + 1)
    }
  })

  test('管理者ダッシュボードがスマホ幅で横スクロールしない', async ({ page }) => {
    await setupAuth(page, TEST_ADMIN)
    await page.goto('/admin/dashboard')
    await settle(page)

    for (const width of PHONE_WIDTHS) {
      await page.setViewportSize({ width, height: 844 })
      await page.waitForTimeout(250)

      // 検索・並び替え・学校フィルターを横並びのままにすると 390px で 468px になる。
      // 表自体の横スクロールは許すが、ページ全体へ伝播させない。
      const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth)
      expect(scrollWidth, `${width}px でページが横スクロールしない`).toBeLessThanOrEqual(width + 1)
    }
  })
})
