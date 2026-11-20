/**
 * @jest-environment jsdom
 */
import { act, renderHook, waitFor } from '@testing-library/react'
import { CHAT_UNSENT_DRAFT_KEY, useMuiChat } from '@/components/mui-chat/hooks/useMuiChat'
import { UserFacingApiError } from '@/lib/user-facing-error'

const mockReplace = jest.fn()
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: mockReplace }),
}))

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, is_guest: false }),
    getUserFetchHeaders: () => ({}),
    logout: jest.fn(),
  },
}))

const mockSendChatMessage = jest.fn()
jest.mock('@/lib/api', () => ({
  sendChatMessage: (...args: unknown[]) => mockSendChatMessage(...args),
  getChatHistory: jest.fn(async () => []),
}))

// #2213 のレビュー: 502〜504 でエラー画面へ移ると、入力欄を空にした文章が消えていた（バックエンドに届いていなければ
// 履歴にも残らない）。タブの中に取っておき、戻ったときに入力欄へ戻す（自動では送り直さない）。
describe('useMuiChat 接続エラーで送れなかった文章', () => {
  beforeEach(() => {
    sessionStorage.clear()
    localStorage.clear()
    mockReplace.mockClear()
    mockSendChatMessage.mockReset()
    jest.spyOn(console, 'error').mockImplementation(() => {})
  })
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('送信が 502 で失敗したら、文章をタブの中に取っておき、エラー画面へ移る', async () => {
    mockSendChatMessage.mockRejectedValue(new UserFacingApiError('接続できません', 502))
    const { result } = renderHook(() => useMuiChat())
    await waitFor(() => expect(result.current.mounted).toBe(true))

    await act(async () => {
      await result.current.handleSend('バックエンドの開発がしたいです')
    })

    expect(sessionStorage.getItem(CHAT_UNSENT_DRAFT_KEY)).toBe('バックエンドの開発がしたいです')
    expect(mockReplace).toHaveBeenCalledWith(expect.stringMatching(/^\/error\?code=502&from=/))
  })

  it('戻ってきたら、取っておいた文章を入力欄へ戻し、取っておいた分は消す（送り直しはしない）', async () => {
    sessionStorage.setItem(CHAT_UNSENT_DRAFT_KEY, 'バックエンドの開発がしたいです')
    const { result } = renderHook(() => useMuiChat())
    await waitFor(() => expect(result.current.input).toBe('バックエンドの開発がしたいです'))
    expect(sessionStorage.getItem(CHAT_UNSENT_DRAFT_KEY)).toBeNull()
    expect(mockSendChatMessage).not.toHaveBeenCalled()
  })

  it('502〜504 以外の失敗では文章を取っておかず、画面の中で失敗を知らせる', async () => {
    mockSendChatMessage.mockRejectedValue(new UserFacingApiError('処理に失敗しました', 500))
    const { result } = renderHook(() => useMuiChat())
    await waitFor(() => expect(result.current.mounted).toBe(true))

    await act(async () => {
      await result.current.handleSend('こんにちは')
    })

    expect(sessionStorage.getItem(CHAT_UNSENT_DRAFT_KEY)).toBeNull()
    expect(mockReplace).not.toHaveBeenCalledWith(expect.stringMatching(/^\/error/))
  })
})
