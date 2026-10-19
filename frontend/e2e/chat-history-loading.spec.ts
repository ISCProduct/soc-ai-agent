import { test, expect, type Page } from '@playwright/test'

/**
 * 履歴の読み込み中に吹き出しの骨格（Skeleton）が出ることを、実ブラウザで確認する。
 *
 * 履歴が0件でも挨拶メッセージが入るので、messages が空なのは初回取得中だけ。
 * 取得は速く一瞬で終わるため、履歴APIをわざと遅らせて捉える。
 *
 * fixtures/auth の偽トークンは使わない。`/` はサーバーコンポーネントで
 * requireSessionUser() を通すため、偽トークンでは /login へ戻されて
 * チャットがマウントされず、履歴の取得自体が起きない。
 * ここでは実際にログインして本物のセッションを作る。
 */

/** 履歴の応答を遅らせる時間。骨格を確認し切れる長さにする。 */
const HISTORY_DELAY_MS = 5000

const EMPTY_HISTORY = { status: 200, contentType: 'application/json', body: '[]' }

/** ローカル検証用の管理者。CI では環境変数で差し替える。 */
const EMAIL = process.env.E2E_EMAIL ?? 'uicheck.local@example.com'
const PASSWORD = process.env.E2E_PASSWORD ?? ''

async function login(page: Page) {
  await page.goto('/login')
  await page.getByRole('textbox', { name: 'メールアドレス' }).fill(EMAIL)
  await page.locator('input[type="password"]').fill(PASSWORD)
  await page.getByRole('button', { name: 'ログイン', exact: true }).click()
  // Cookie の設定を待ってから遷移する実装なので、/ へ着いたらセッションは有効。
  await page.waitForURL('**/', { timeout: 30000 })
}

/** チャットの入力欄が出れば、チャットが描画されている。 */
async function expectChatVisible(page: Page) {
  await expect(page.getByPlaceholder(/メッセージ|入力/).first()).toBeVisible({ timeout: 60000 })
}

test.describe('チャット履歴の読み込み表示', () => {
  test.skip(!PASSWORD, 'E2E_PASSWORD が未設定のため実行しない')

  test('読み込み中は骨格を出し、届いたら本文に入れ替わる', async ({ page }) => {
    await login(page)
    await expectChatVisible(page)

    // 履歴だけ遅らせて読み込み直す
    await page.route('**/api/chat/history*', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, HISTORY_DELAY_MS))
      await route.fulfill(EMPTY_HISTORY)
    })
    await page.reload()

    const skeletons = page.locator('.MuiSkeleton-root')
    await expect(skeletons.first()).toBeVisible({ timeout: 60000 })
    await page.screenshot({ path: 'test-results/chat-loading-skeleton.png' })

    // 届いたら骨格が消えて本文が出る
    await expect(skeletons).toHaveCount(0, { timeout: 30000 })
    await page.screenshot({ path: 'test-results/chat-loaded.png' })
  })

  test('履歴の取得に失敗したときは骨格ではなくエラーを出す', async ({ page }) => {
    await login(page)
    await expectChatVisible(page)

    // 骨格を出し続けると、失敗したのに読み込み中に見える。
    await page.route('**/api/chat/history*', (route) =>
      route.fulfill({ status: 500, contentType: 'application/json', body: '{}' }),
    )
    await page.reload()
    await page.waitForTimeout(4000)

    await expect(page.locator('.MuiSkeleton-root')).toHaveCount(0)
    await page.screenshot({ path: 'test-results/chat-history-error.png' })
  })
})
