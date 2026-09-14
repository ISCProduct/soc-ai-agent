import { test, expect, type Page } from '@playwright/test'
import {
  CHAT_SESSION_ID,
  INTERVIEW_MCQ,
  historyItem,
  mockChatHistory,
  mockEmptyChatAuxRoutes,
  setupChatCriticalAuth,
} from './fixtures/chat'

async function mockChatPost(
  page: Page,
  handler: (body: { message?: string; job_category_id?: number }) => {
    status?: number
    body: Record<string, unknown>
  },
) {
  // /api/chat のみ（/api/chat/history 等は別モックに任せる）
  await page.route(/\/api\/chat\/?(?:\?.*)?$/, async (route) => {
    if (route.request().method() !== 'POST') {
      await route.continue()
      return
    }
    const body = route.request().postDataJSON() as {
      message?: string
      job_category_id?: number
    }
    const result = handler(body)
    await route.fulfill({
      status: result.status ?? 200,
      contentType: 'application/json',
      body: JSON.stringify(result.body),
    })
  })
}

/**
 * チャット主要機能の回帰 E2E。
 * - 経験談MCQで「A」を選んでも職種clarificationに飛ばない
 * - 無効回答警告後も元の選択肢ボタンが残る
 * - Enter単独は送信せず Ctrl/Meta+Enter で送信
 * - job_category_id が後続リクエストに載る
 */
/**
 * チャットの入力欄。placeholder は状態で変わる（#1318 で選択肢まわりが増えた）。
 *   - 通常: メッセージを入力...
 *   - 選択肢表示中（未選択）: 選択肢を選ぶか、同じ内容を入力してください
 *   - 選択肢を選んだ後: 任意: そう思う理由を一言（空でも送信可）
 *   - その他を選択: その他の内容を入力...
 * 個別のテストが文言に依存しないよう、ここに集約する。
 */
function chatInput(page: Page) {
  return page.getByPlaceholder(/メッセージを入力|選択肢を選ぶか|そう思う理由|その他の内容/)
}

test.describe('チャット主要機能（職種・選択肢・送信）', () => {
  test.beforeEach(async ({ page }) => {
    await setupChatCriticalAuth(page)
    await mockEmptyChatAuxRoutes(page)
  })

  test('経験談MCQの選択肢Aは職種判定メッセージを出さず送信される', async ({ page }) => {
    await mockChatHistory(page, [
      historyItem(1, 'assistant', 'これまでの経験を具体的に教えてください。'),
      historyItem(
        2,
        'user',
        '児童養護施設の案件でIT弱者の職員が使いやすいUIを作りました',
      ),
      historyItem(3, 'assistant', INTERVIEW_MCQ),
    ])

    const posted: Array<{ message?: string; job_category_id?: number }> = []
    await mockChatPost(page, (body) => {
      posted.push(body)
      return {
        body: {
          response: 'なるほど、要件の曖昧さが一番の課題だったんですね。次の質問です。',
          is_complete: false,
          total_questions: 15,
          answered_questions: 2,
          job_category_id: 0,
        },
      }
    })

    await page.goto('/')
    await expect(page.getByText('一番モヤっとした', { exact: false })).toBeVisible({
      timeout: 15000,
    })
    await expect(page.getByRole('button', { name: /A\.\s*要件が曖昧/ })).toBeVisible()

    // 選択肢のクリックは「選択」まで。理由を書いてから送れるよう、ここでは送信しない
    await page.getByRole('button', { name: /A\.\s*要件が曖昧/ }).click()
    expect(posted).toHaveLength(0)

    await page.getByRole('button', { name: 'メッセージを送信' }).click()

    await expect
      .poll(() => posted.length, { timeout: 10000 })
      .toBe(1)
    expect(posted[0]?.message).toBe('A')
    await expect(page.getByText('職種を特定できません')).toHaveCount(0)
    await expect(page.getByText('要件の曖昧さが一番の課題')).toBeVisible()
  })

  // 選択肢を選んだあとに理由を書くと "A: 理由" で送られる（#1318 の主目的）。
  // buildChoiceOutgoingMessage の単体テストはあるが、画面の配線を守るものが無かった。
  test('選択肢を選んで理由を入力すると「記号: 理由」で送信される', async ({ page }) => {
    await mockChatHistory(page, [historyItem(1, 'assistant', INTERVIEW_MCQ)])

    const posted: Array<{ message?: string }> = []
    await mockChatPost(page, (body) => {
      posted.push(body)
      return {
        body: {
          response: '理由まで書けていますね。次の質問です。',
          is_complete: false,
          total_questions: 15,
          answered_questions: 1,
          job_category_id: 0,
        },
      }
    })

    await page.goto('/')
    await expect(page.getByRole('button', { name: /A\.\s*要件が曖昧/ })).toBeVisible({
      timeout: 15000,
    })

    await page.getByRole('button', { name: /A\.\s*要件が曖昧/ }).click()
    await chatInput(page).fill('仕様が固まる前に実装を始めたため')

    await page.getByRole('button', { name: 'メッセージを送信' }).click()

    await expect.poll(() => posted.length, { timeout: 10000 }).toBe(1)
    expect(posted[0]?.message).toBe('A: 仕様が固まる前に実装を始めたため')
  })

  // 何も選ばず本文も空なら送信できない（誤送信の防止）
  test('未選択かつ本文が空なら送信ボタンは押せない', async ({ page }) => {
    await mockChatHistory(page, [historyItem(1, 'assistant', INTERVIEW_MCQ)])

    const posted: Array<{ message?: string }> = []
    await mockChatPost(page, (body) => {
      posted.push(body)
      return { body: { response: 'ok', is_complete: false, total_questions: 15, answered_questions: 1 } }
    })

    await page.goto('/')
    await expect(page.getByRole('button', { name: /A\.\s*要件が曖昧/ })).toBeVisible({
      timeout: 15000,
    })

    await expect(page.getByRole('button', { name: 'メッセージを送信' })).toBeDisabled()
    expect(posted).toHaveLength(0)
  })

  test('無効回答警告のあとでも直前質問の選択肢ボタンが残る', async ({ page }) => {
    await mockChatHistory(page, [
      historyItem(1, 'assistant', INTERVIEW_MCQ),
      historyItem(2, 'user', 'あ'),
      historyItem(
        3,
        'assistant',
        '書かれた内容にはお答えできません。質問に回答してください。（1/3回目の警告）',
      ),
    ])

    await page.goto('/')
    await expect(page.getByText('1/3回目の警告')).toBeVisible({ timeout: 15000 })
    await expect(page.getByRole('button', { name: /A\.\s*要件が曖昧/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /B\.\s*技術制約/ })).toBeVisible()
  })

  test('Enter単独では送信せず Ctrl+Enter で送信する', async ({ page }) => {
    await mockChatHistory(page, [])

    const posted: string[] = []
    await mockChatPost(page, (body) => {
      posted.push(body.message ?? '')
      return {
        body: {
          response: '受け取りました',
          is_complete: false,
          total_questions: 15,
          answered_questions: 1,
        },
      }
    })

    await page.goto('/')
    const input = chatInput(page)
    await expect(input).toBeVisible({
      timeout: 15000,
    })
    await input.fill('Webエンジニアに興味があります')
    await input.press('Enter')
    await page.waitForTimeout(400)
    expect(posted.length).toBe(0)

    await input.press('Control+Enter')
    await expect
      .poll(() => posted.length, { timeout: 10000 })
      .toBe(1)
    expect(posted[0]).toBe('Webエンジニアに興味があります')
  })

  test('レスポンスの job_category_id が次の送信に載る', async ({ page }) => {
    await mockChatHistory(page, [
      historyItem(1, 'assistant', 'どのようなIT職種に興味がありますか？'),
    ])

    const posted: number[] = []
    let call = 0
    await mockChatPost(page, (body) => {
      posted.push(body.job_category_id ?? 0)
      call += 1
      if (call === 1) {
        return {
          body: {
            response:
              'ありがとうございます。次の質問です。\nチームで困った経験を教えてください。',
            is_complete: false,
            total_questions: 15,
            answered_questions: 1,
            job_category_id: 3,
          },
        }
      }
      return {
        body: {
          response: 'なるほど、詳しく聞けて助かります。',
          is_complete: false,
          total_questions: 15,
          answered_questions: 2,
          job_category_id: 3,
        },
      }
    })

    await page.goto('/')
    await expect(page.getByText('IT職種に興味がありますか', { exact: false })).toBeVisible({
      timeout: 15000,
    })

    const input = chatInput(page)
    await input.fill('Webエンジニア')
    await input.press('Control+Enter')
    await expect(page.getByText('次の質問です', { exact: false })).toBeVisible({
      timeout: 10000,
    })

    await input.fill('要件が曖昧で調整に苦労しました')
    await input.press('Control+Enter')
    await expect
      .poll(() => posted.length, { timeout: 10000 })
      .toBe(2)

    expect(posted[0]).toBe(0)
    expect(posted[1]).toBe(3)

    const stored = await page.evaluate((sid) => {
      return sessionStorage.getItem(`chat_job_category_id_${sid}`)
    }, CHAT_SESSION_ID)
    expect(stored).toBe('3')
  })
})
