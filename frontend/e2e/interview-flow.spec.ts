import { test, expect, type Page, type Route } from '@playwright/test'
import { setupAuth, TEST_USER } from './fixtures/auth'

/**
 * AI 面接の本編フロー（開始 → Q&A → 提出 → レポート）の E2E（SOCAIAGENT-592 / #2043）。
 *
 * CI にはバックエンドも OpenAI も無いので、面接の API はすべてここでモックする。
 * /start-turn・/turn は本番と同じ multipart（JSON のメタ ＋ 音声）で返し、画面の解析
 * （parseMultipartResponse）から再生・発話ログへの反映までを通す。マイクとカメラは
 * Chromium の偽のデバイスを使い、録音（MediaRecorder）も実物で動かす。
 */

test.use({
  launchOptions: {
    args: [
      '--use-fake-ui-for-media-stream',
      '--use-fake-device-for-media-stream',
      // 面接官の音声（AudioContext / Audio 要素）は、自動実行だとユーザー操作の扱いにならず止まる
      '--autoplay-policy=no-user-gesture-required',
    ],
  },
  permissions: ['microphone', 'camera'],
})

// 面接画面は大きく、開発サーバーの初回コンパイルに時間がかかる
test.setTimeout(120_000)

const SESSION_ID = 101
const COMPANY = { id: 7, name: 'E2Eテスト株式会社', industry: '情報通信業', location: '東京都', description: '' }

const QUESTION_1 = '本日はよろしくお願いします。まず自己紹介をお願いします。'
const ANSWER_1 = '情報系の学科で、チーム開発の授業でリーダーを務めました。'
const QUESTION_2 = 'リーダーとして工夫したことを教えてください。'

const REPORT = {
  session_id: SESSION_ID,
  summary_text: '結論から話せており、具体的な経験に触れられていました。',
  scores_json: JSON.stringify({ logic: 4, specificity: 3, ownership: 4, communication: 4 }),
  evidence_json: '{}',
  strengths_json: JSON.stringify(['結論から話せている']),
  improvements_json: JSON.stringify(['数字で成果を示す']),
  created_at: '2026-11-13T05:00:00Z',
  updated_at: '2026-11-13T05:00:00Z',
}

/** 0.1 秒の無音の WAV。ブラウザが実際に再生できる音声を返す。 */
function silentWav(): Buffer {
  const sampleRate = 8000
  const samples = sampleRate / 10
  const data = Buffer.alloc(samples * 2)
  const header = Buffer.alloc(44)
  header.write('RIFF', 0)
  header.writeUInt32LE(36 + data.length, 4)
  header.write('WAVE', 8)
  header.write('fmt ', 12)
  header.writeUInt32LE(16, 16)
  header.writeUInt16LE(1, 20)
  header.writeUInt16LE(1, 22)
  header.writeUInt32LE(sampleRate, 24)
  header.writeUInt32LE(sampleRate * 2, 28)
  header.writeUInt16LE(2, 32)
  header.writeUInt16LE(16, 34)
  header.write('data', 36)
  header.writeUInt32LE(data.length, 40)
  return Buffer.concat([header, data])
}

/** /start-turn・/turn と同じ形の multipart（1つ目が JSON のメタ、2つ目が音声）を返す。 */
async function fulfillTurn(route: Route, meta: Record<string, unknown>) {
  const boundary = 'e2e-turn-boundary'
  const body = Buffer.concat([
    Buffer.from(`--${boundary}\r\nContent-Type: application/json\r\n\r\n${JSON.stringify(meta)}\r\n`),
    Buffer.from(`--${boundary}\r\nContent-Type: audio/wav\r\n\r\n`),
    silentWav(),
    Buffer.from(`\r\n--${boundary}--\r\n`),
  ])
  await route.fulfill({ status: 200, headers: { 'Content-Type': `multipart/mixed; boundary=${boundary}` }, body })
}

type Options = {
  /** レポートを返すまでに「生成中（report なし）」を返す回数 */
  pendingPolls?: number
  /** 「生成中」の前にレポートの取得を失敗させる回数 */
  failedPolls?: number
  /** /turn を失敗させる */
  turnFails?: boolean
}

async function mockInterviewApis(page: Page, opts: Options = {}) {
  const calls = { turn: 0, finish: 0, detail: 0 }
  const json = (route: Route, body: unknown, status = 200) =>
    route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })

  await page.route('**/api/companies/interview-hints**', (route) => json(route, { hints: [] }))
  await page.route('**/api/companies?**', (route) => json(route, { companies: [COMPANY], total: 1 }))
  await page.route('**/api/user/weight-scores**', (route) => json(route, { weight_scores: [] }))
  await page.route('**/api/notifications**', (route) => json(route, { notifications: [], unread_count: 0 }))

  await page.route('**/api/interviews**', async (route) => {
    const req = route.request()
    const path = new URL(req.url()).pathname
    const method = req.method()
    const session = { id: SESSION_ID, user_id: TEST_USER.user_id, status: 'in_progress', language: 'ja', created_at: '2026-11-13T05:00:00Z' }

    if (path === '/api/interviews' && method === 'POST') return json(route, session)
    if (path === `/api/interviews/${SESSION_ID}/start`) return json(route, { ...session, status: 'in_progress' })
    if (path === `/api/interviews/${SESSION_ID}/start-turn`) {
      return fulfillTurn(route, { ai_text: QUESTION_1, total_questions: '3', question_source: 'plan' })
    }
    if (path === `/api/interviews/${SESSION_ID}/turn`) {
      calls.turn++
      if (opts.turnFails) return json(route, { message: 'turn failed' }, 500)
      return fulfillTurn(route, { user_text: ANSWER_1, ai_text: QUESTION_2, total_questions: '3', question_source: 'plan' })
    }
    if (path === `/api/interviews/${SESSION_ID}/finish`) {
      calls.finish++
      return json(route, { ...session, status: 'finished' })
    }
    if (path === `/api/interviews/${SESSION_ID}` && method === 'GET') {
      calls.detail++
      const failed = opts.failedPolls ?? 0
      const pending = opts.pendingPolls ?? 0
      if (calls.detail <= failed) return json(route, { message: 'temporary error' }, 500)
      const ready = calls.detail > failed + pending
      return json(route, { session: { ...session, status: 'finished' }, utterances: [], ...(ready ? { report: REPORT } : {}) })
    }
    // 発話の保存・動画のアップロード・言い換え候補などは成功だけ返す
    return json(route, {})
  })
  return calls
}

/** 企業を選んでロビーへ進み、同意して面接に参加する。最初の質問が出るまで待つ。 */
async function joinInterview(page: Page) {
  await page.goto('/interview', { timeout: 60000 })
  await page.getByRole('button', { name: COMPANY.name }).click({ timeout: 30000 })
  await page.getByRole('button', { name: '面接を開始する' }).click()

  await page.getByRole('button', { name: '面接に参加' }).click({ timeout: 30000 })
  const consent = page.getByRole('dialog')
  await consent.getByLabel('上記の内容に同意して面接練習を始める').check()
  await consent.getByRole('button', { name: '面接に参加' }).click()

  await expect(page.getByText(QUESTION_1).first()).toBeVisible({ timeout: 30000 })
}

/** 「話す」で録音を始め、少し話してから送信する。 */
async function answerOnce(page: Page) {
  const talk = page.getByRole('button', { name: /話す/ })
  await expect(talk).toBeEnabled({ timeout: 30000 })
  await talk.click()
  await expect(page.getByRole('button', { name: /録音中/ })).toBeVisible()
  // 偽のマイクの音声が録音に溜まるのを待つ（録音が空だと送信しない）
  await page.waitForTimeout(1500)
  await page.getByRole('button', { name: /録音中/ }).click()
}

test.describe('AI 面接の本編フロー', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuth(page)
    // CI のコンテナには音声の出力先が無く、面接官の音声（Audio 要素）が「再生終了」を出さずに
    // 止まることがある（「面接官が発言中」のまま次へ進めない）。音の出力はこの E2E の対象では
    // ないので、音声の要素だけ、少し待って再生終了を出すようにする（カメラの video 要素は触らない）。
    await page.addInitScript(() => {
      // 出力先が無いと AudioContext.resume() が終わらないことがあり、play() まで進まない
      AudioContext.prototype.resume = function () {
        return Promise.resolve()
      }
      const original = HTMLMediaElement.prototype.play
      HTMLMediaElement.prototype.play = function (this: HTMLMediaElement) {
        if (!(this instanceof HTMLAudioElement)) return original.call(this)
        const el = this
        window.setTimeout(() => el.dispatchEvent(new Event('ended')), 200)
        return Promise.resolve()
      }
    })
  })

  test('開始 → 1問回答 → 次の質問 → 終了 → レポートの生成を待って表示する', async ({ page }) => {
    const calls = await mockInterviewApis(page, { pendingPolls: 1 })
    await joinInterview(page)

    await answerOnce(page)
    await expect(page.getByText(QUESTION_2).first()).toBeVisible({ timeout: 30000 })
    // 発話ログには、最初の質問・サーバーが文字起こしした回答・次の質問の3件が積まれる
    const log = page.getByRole('button', { name: 'これまでの質問・回答（3）' })
    await expect(log).toBeVisible()
    await log.click()
    await expect(page.getByText(ANSWER_1).first()).toBeVisible()
    expect(calls.turn).toBe(1)

    await page.getByRole('button', { name: '終了' }).first().click()
    // 終了 API を待たずにレポート画面へ進み、生成中を出す
    await expect(page.getByText('レポートを生成中です...')).toBeVisible({ timeout: 15000 })
    // ポーリング（3秒ごと）で2回目にレポートが届く
    await expect(page.getByText(REPORT.summary_text)).toBeVisible({ timeout: 20000 })
    expect(calls.finish).toBe(1)
  })

  test('レポートの取得が続けて失敗したら再試行を出し、押すと表示できる', async ({ page }) => {
    // 3回続けて失敗するとエラー（REPORT_POLL_MAX_CONSECUTIVE_FAILURES）。4回目以降は成功する
    await mockInterviewApis(page, { failedPolls: 3 })
    await joinInterview(page)
    await page.getByRole('button', { name: '終了' }).first().click()

    await expect(page.getByText('レポート生成に失敗しました。')).toBeVisible({ timeout: 30000 })
    await page.getByRole('button', { name: '再試行' }).click()
    await expect(page.getByText(REPORT.summary_text)).toBeVisible({ timeout: 20000 })
  })

  test('回答の送信が失敗したら、面接を止めずにエラーを知らせる', async ({ page }) => {
    const calls = await mockInterviewApis(page, { turnFails: true })
    await joinInterview(page)

    await answerOnce(page)
    await expect.poll(() => calls.turn, { timeout: 15000 }).toBe(1)
    await expect(page.getByRole('alert').first()).toBeVisible({ timeout: 15000 })
    // 面接は続けられる（もう一度話せる）
    await expect(page.getByRole('button', { name: /話す/ })).toBeEnabled({ timeout: 15000 })
    await expect(page.getByText(QUESTION_2)).toHaveCount(0)
    // 発話ログは最初の質問だけ（失敗したターンの発話は積まない）
    await expect(page.getByRole('button', { name: 'これまでの質問・回答（1）' })).toBeVisible()
  })
})
