/** @jest-environment jsdom */

import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { UserFacingApiError, userFacingApiMessage, CHAT_TOO_LONG_MESSAGE } from '@/lib/user-facing-error'

/**
 * サーバーが長すぎる本文を 400 で断ったとき、汎用の失敗ではなく原因を出す（SOCAIAGENT-702）。
 */

const sendChatMessage = jest.fn()
const getChatHistory = jest.fn()

jest.mock('@/lib/api', () => ({
  sendChatMessage: (...a: unknown[]) => sendChatMessage(...a),
  getChatHistory: (...a: unknown[]) => getChatHistory(...a),
}))
jest.mock('@/lib/auth', () => ({
  authService: { getStoredUser: () => ({ user_id: 1, name: 'テスト' }), logout: jest.fn() },
}))
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn() }),
}))

import { MuiChat } from '@/components/mui-chat'

describe('長すぎる本文の 400', () => {
  it('400 の本文に上限超えの文言があれば、その文言を返す', () => {
    expect(userFacingApiMessage(400, '{"message":"文字数が上限を超えています"}')).toBe(CHAT_TOO_LONG_MESSAGE)
    expect(userFacingApiMessage(400, '{"message":"Missing required fields"}')).not.toBe(CHAT_TOO_LONG_MESSAGE)
  })

  it('チャット画面は汎用の失敗ではなく原因を出す', async () => {
    Element.prototype.scrollIntoView = jest.fn()
    sessionStorage.clear()
    localStorage.clear()
    getChatHistory.mockResolvedValue([
      { id: 1, role: 'assistant', content: '自由に教えてください。', created_at: '2026-10-14T00:00:00Z' },
    ])
    sendChatMessage.mockRejectedValue(new UserFacingApiError(CHAT_TOO_LONG_MESSAGE, 400))
    render(<MuiChat />)

    const box = await screen.findByRole('textbox')
    fireEvent.change(box, { target: { value: 'こんにちは' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'メッセージを送信' }))
    })
    await waitFor(() => expect(sendChatMessage).toHaveBeenCalled())
    expect(await screen.findByText(new RegExp(CHAT_TOO_LONG_MESSAGE))).toBeInTheDocument()
    expect(screen.queryByText(/送信に失敗しました/)).not.toBeInTheDocument()
  })
})
