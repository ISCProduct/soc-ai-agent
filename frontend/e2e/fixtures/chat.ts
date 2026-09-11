import { Page } from '@playwright/test'
import { setupAuth, TEST_USER } from './auth'

export const CHAT_SESSION_ID = 'e2e-chat-critical-session'

export type ChatHistoryItem = {
  id: number
  session_id: string
  user_id: number
  role: 'user' | 'assistant'
  content: string
  created_at: string
}

export const INTERVIEW_MCQ = [
  '児童養護施設向けUIの経験、素敵だと思います。',
  '一番モヤっとした／妥協せざるを得なかったのはどれですか？（一番近いものを選んでください）',
  '',
  'A) 要件が曖昧でスコープが膨らむ',
  'B) 技術制約でやりたいUIが出せない',
  'C) ステークホルダー調整が難しい',
].join('\n')

export async function setupChatCriticalAuth(page: Page) {
  await setupAuth(page, TEST_USER)
  await page.addInitScript((sessionId: string) => {
    sessionStorage.setItem('chatSessionId', sessionId)
    localStorage.setItem('currentSessionId', sessionId)
  }, CHAT_SESSION_ID)
}

export async function mockChatHistory(page: Page, messages: ChatHistoryItem[]) {
  await page.route('**/api/chat/history*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(messages),
    })
  })
}

export async function mockEmptyChatAuxRoutes(page: Page) {
  await page.route('**/api/chat/session', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ session_id: CHAT_SESSION_ID }),
      })
      return
    }
    await route.continue()
  })

  await page.route('**/api/chat/messages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ messages: [] }),
    })
  })
}

export function historyItem(
  id: number,
  role: 'user' | 'assistant',
  content: string,
): ChatHistoryItem {
  return {
    id,
    session_id: CHAT_SESSION_ID,
    user_id: TEST_USER.user_id,
    role,
    content,
    created_at: new Date(Date.UTC(2026, 0, 1, 0, 0, id)).toISOString(),
  }
}
