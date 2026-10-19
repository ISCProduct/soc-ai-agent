/** @jest-environment jsdom */

import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'

/**
 * 選択肢ボタンは「直近のアシスタント質問」から毎回組み立て直す。
 *
 * 選択肢が質問ごとに変わらない不具合（固定の5択がフェーズ中ずっと並ぶ）は
 * バックエンド側の原因だったが、画面側も同じ症状を作れる位置にいる。
 * ここで入れ替わり・消え方を固定しておく。
 */

const sendChatMessage = jest.fn()
const getChatHistory = jest.fn()

jest.mock('@/lib/api', () => ({
  sendChatMessage: (...a: unknown[]) => sendChatMessage(...a),
  getChatHistory: (...a: unknown[]) => getChatHistory(...a),
}))

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, name: 'テスト' }),
    logout: jest.fn(),
  },
}))

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn() }),
}))

import { MuiChat } from '@/components/mui-chat'

const Q1 = `新しいことに挑戦しましたか？

A) 自分から新しいことを学び続けたい
B) 必要になったときに学べばいい
C) 今できることを深めたい
D) その他（自由記述）`

const Q2 = `細かいところまで気を配るタイプですか？

A) 細かいところまで確認してから進める
B) 大事なところは必ず見直す
C) 全体像を優先して細部は後から直す
D) その他（自由記述）`

/** 無効回答の案内。質問自体は変わらないので選択肢も変わってはいけない。 */
const WARNING = '質問に沿った内容でもう一度お願いします。'

/** フェーズの最初の1問は自由記述で来る。選択肢は出してはいけない。 */
const FREE_TEXT = '将来のキャリアで諦めたことや譲れないことを教えてください。'

async function answerFirstChoice() {
  await screen.findByRole('button', { name: /自分から新しいことを学び続けたい/ })
  fireEvent.click(screen.getByRole('button', { name: /自分から新しいことを学び続けたい/ }))
  await act(async () => {
    fireEvent.click(screen.getByRole('button', { name: 'メッセージを送信' }))
  })
  await waitFor(() => expect(sendChatMessage).toHaveBeenCalled())
}

describe('チャットの選択肢の切り替わり', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    Element.prototype.scrollIntoView = jest.fn()
    sessionStorage.clear()
    localStorage.clear()
    getChatHistory.mockResolvedValue([
      { id: 1, role: 'assistant', content: Q1, created_at: '2026-10-14T00:00:00Z' },
    ])
  })

  it('次の質問が来たら、その質問の選択肢に入れ替わる', async () => {
    sendChatMessage.mockResolvedValue({ response: Q2, is_complete: false })
    render(<MuiChat />)
    await answerFirstChoice()

    await screen.findByRole('button', { name: /細かいところまで確認してから進める/ })
    expect(
      screen.queryByRole('button', { name: /自分から新しいことを学び続けたい/ }),
    ).toBeNull()
  })

  it('次が自由記述の質問なら選択肢は消える', async () => {
    sendChatMessage.mockResolvedValue({ response: FREE_TEXT, is_complete: false })
    render(<MuiChat />)
    await answerFirstChoice()

    await waitFor(() =>
      expect(
        screen.queryByRole('button', { name: /自分から新しいことを学び続けたい/ }),
      ).toBeNull(),
    )
  })

  it('無効回答の案内では、同じ質問の選択肢を残す', async () => {
    sendChatMessage.mockResolvedValue({ response: WARNING, is_complete: false })
    render(<MuiChat />)
    await answerFirstChoice()

    expect(
      screen.getByRole('button', { name: /自分から新しいことを学び続けたい/ }),
    ).toBeTruthy()
  })
})
