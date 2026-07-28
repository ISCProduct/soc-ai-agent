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

    await page.getByRole('button', { name: /A\.\s*要件が曖昧/ }).click()

    await expect
      .poll(() => posted.length, { timeout: 10000 })
      .toBe(1)
    expect(posted[0]?.message).toBe('A')
    await expect(page.getByText('職種を特定できません')).toHaveCount(0)
    await expect(page.getByText('要件の曖昧さが一番の課題')).toBeVisible()
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
    await expect(page.getByText('キャリアエージェント', { exact: false })).toBeVisible({
      timeout: 15000,
    })

    const input = page.getByPlaceholder(/メッセージを入力|選択肢と同じ内容/)
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

    const input = page.getByPlaceholder(/メッセージを入力|選択肢と同じ内容/)
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
