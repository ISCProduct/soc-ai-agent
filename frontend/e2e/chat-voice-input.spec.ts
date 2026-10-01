import { test, expect, type Page } from '@playwright/test'

/**
 * 音声入力を、マイクを許可した実ブラウザで確かめる。
 *
 * Chrome の fake device にWAVを流し込んで「話した」状態を作る。
 * 実機で声を出す代わりになるが、Web Speech API は Chrome がクラウドの
 * 音声認識へ投げるため、自動実行の環境では応答が返らないことがある。
 * 認識結果まで取れなかった場合は、許可とボタンの状態までを確認する。
 */

const EMAIL = process.env.E2E_EMAIL ?? 'uicheck.local@example.com'
const PASSWORD = process.env.E2E_PASSWORD ?? ''

async function login(page: Page) {
  await page.goto('/login')
  await page.getByRole('textbox', { name: 'メールアドレス' }).fill(EMAIL)
  await page.locator('input[type="password"]').fill(PASSWORD)
  await page.getByRole('button', { name: 'ログイン', exact: true }).click()
  await page.waitForURL('**/', { timeout: 30000 })
}

// 実機で毎回ダイアログを出させないよう、テスト側で許可を与える。
// launchOptions は describe 内に置けない（worker が分かれるため）。
test.use({
  permissions: ['microphone'],
  launchOptions: {
    args: [
      // 合成マイクを使う。実際の音声ファイルは流さない。
      // Chrome の音声認識はクラウドへ投げるため自動実行では応答が返らず、
      // 音声を用意しても結果は変わらない（200KBのWAVを置く意味が無い）。
      '--use-fake-ui-for-media-stream',
      '--use-fake-device-for-media-stream',
    ],
  },
})

test.describe('音声入力', () => {
  test.skip(!PASSWORD, 'E2E_PASSWORD が未設定のため実行しない')

  test('マイクを許可すると聞き取りに入り、応答が無ければ自分で畳む', async ({ page }) => {
    await login(page)

    const mic = page.getByRole('button', { name: '音声で入力する' })
    await expect(mic).toBeVisible({ timeout: 30000 })
    await expect(mic).toHaveAttribute('aria-pressed', 'false')

    await mic.click()

    // 聞き取り中はラベルと押下状態が入れ替わる（読み上げにも伝わる）
    const stopMic = page.getByRole('button', { name: '音声入力を止める' })
    await expect(stopMic).toBeVisible({ timeout: 10000 })
    await expect(stopMic).toHaveAttribute('aria-pressed', 'true')
    await page.screenshot({ path: 'test-results/chat-voice-listening.png' })

    // Chrome の音声認識はクラウドへ投げるため、自動実行では応答が返らない。
    // 放置するとマイクを掴み続けるので、時間で畳めることをここで確かめる。
    await expect(page.getByRole('button', { name: '音声で入力する' })).toBeVisible({
      timeout: 25000,
    })
    await expect(page.getByRole('status')).toContainText('キーボード')
    await page.screenshot({ path: 'test-results/chat-voice-recovered.png' })
  })

  test('押してもメッセージは送られない', async ({ page }) => {
    // 誤認識のまま送られると学生は直せない。送信は本人の操作に委ねる。
    await login(page)
    const mic = page.getByRole('button', { name: '音声で入力する' })
    await expect(mic).toBeVisible({ timeout: 30000 })

    // 挨拶が出てから数える
    await expect(page.locator('[role="log"] article')).toHaveCount(1, { timeout: 20000 })
    await mic.click()
    await page.waitForTimeout(5000)
    await expect(page.locator('[role="log"] article')).toHaveCount(1)
  })
})
