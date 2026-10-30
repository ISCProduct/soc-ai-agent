import { test, expect } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

// プロフィールの見出しと本文の左右端が揃うことの回帰テスト（#1767）。
//
// ヘッダー帯が px を持ち、その中の <Container maxWidth="lg"> の gutter と二重になっていた。
// Container は最大1200pxで中央寄せされるため、ずれは帯の内容幅が1200pxを下回る
// 画面幅（およそ1248px未満）でだけ出る。修正前の実測値:
//   390px:  見出し x=16+16=32 / 本文 x=16
//   1000px: 見出し x=24+24=48 / 本文 x=24（右端も同様に24px内側）
//   1440px: 見出し・本文とも x=144 で修正前から一致する（中央寄せのため。2000px も同様）。広い幅が退行しないことの確認用
//
// responsive-layout.spec.ts と同種の検査だが、同ファイルは #1781 で変更中のため別ファイルに置く。

const WIDTHS = [390, 1000, 1440, 2000]

// CI ではバックエンドが起動していない。未モックの API を実サーバーへ流すと
// 応答を待ち続けて画面が「読み込み中」のまま止まる。空応答で即座に返す。
async function stubApi(page: import('@playwright/test').Page) {
  await page.route('**/api/**', (route) =>
    route.request().method() === 'GET'
      ? route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
      : route.fulfill({ status: 204, body: '' }),
  )
}

test('プロフィールの見出しと本文の左右端が揃う', async ({ page }) => {
  await stubApi(page)
  await setupAuth(page, TEST_USER)

  await page.setViewportSize({ width: 1000, height: 900 })
  await page.goto('/profile')

  const heading = page.getByRole('heading', { level: 1, name: 'プロフィール' })
  // 見出しは帯の中の行（ホームへ戻るボタンと横並び）なので、その行の左右端を測る。
  const headerRow = heading.locator('..')
  const firstCard = page.locator('.MuiCard-root').first()
  await heading.waitFor({ state: 'visible', timeout: 15000 })
  await firstCard.waitFor({ state: 'visible', timeout: 15000 })

  for (const width of WIDTHS) {
    await page.setViewportSize({ width, height: 900 })
    await page.waitForFunction((w: number) => window.innerWidth === w, width, { timeout: 5000 })

    const head = (await headerRow.boundingBox())!
    const body = (await firstCard.boundingBox())!
    expect(head.x, `${width}px: 左端（見出し ${head.x} / 本文 ${body.x}）`).toBeCloseTo(body.x, 0)
    expect(head.x + head.width, `${width}px: 右端（見出し ${head.x + head.width} / 本文 ${body.x + body.width}）`)
      .toBeCloseTo(body.x + body.width, 0)
  }
})
