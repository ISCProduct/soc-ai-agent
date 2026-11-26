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

let mockUser: { user_id: number; is_guest: boolean } | null = { user_id: 1, is_guest: false }
jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => mockUser,
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
    mockUser = { user_id: 1, is_guest: false }
    mockReplace.mockClear()
    mockSendChatMessage.mockReset()
    jest.spyOn(console, 'error').mockImplementation(() => {})
  })
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('送信が 502 で失敗したら、文章を書いた利用者の ID と一緒にタブの中に取っておき、エラー画面へ移る', async () => {
    mockSendChatMessage.mockRejectedValue(new UserFacingApiError('接続できません', 502))
    const { result } = renderHook(() => useMuiChat())
    await waitFor(() => expect(result.current.mounted).toBe(true))

    await act(async () => {
      await result.current.handleSend('バックエンドの開発がしたいです')
    })

    expect(JSON.parse(sessionStorage.getItem(CHAT_UNSENT_DRAFT_KEY) ?? 'null')).toEqual({
      text: 'バックエンドの開発がしたいです',
      userId: 1,
    })
    expect(mockReplace).toHaveBeenCalledWith(expect.stringMatching(/^\/error\?code=502&from=/))
  })

  it('同じ利用者が戻ってきたら、取っておいた文章を入力欄へ戻し、取っておいた分は消す（送り直しはしない）', async () => {
    sessionStorage.setItem(CHAT_UNSENT_DRAFT_KEY, JSON.stringify({ text: 'バックエンドの開発がしたいです', userId: 1 }))
    const { result } = renderHook(() => useMuiChat())
    await waitFor(() => expect(result.current.input).toBe('バックエンドの開発がしたいです'))
    expect(sessionStorage.getItem(CHAT_UNSENT_DRAFT_KEY)).toBeNull()
    expect(mockSendChatMessage).not.toHaveBeenCalled()
  })

  it('別の利用者が同じタブでチャットを開いても、前の利用者の文章は戻さずに消す（共用 PC）', async () => {
    sessionStorage.setItem(CHAT_UNSENT_DRAFT_KEY, JSON.stringify({ text: '前の学生の文章', userId: 1 }))
    mockUser = { user_id: 2, is_guest: false }
    const { result } = renderHook(() => useMuiChat())
    await waitFor(() => expect(result.current.mounted).toBe(true))
    await waitFor(() => expect(sessionStorage.getItem(CHAT_UNSENT_DRAFT_KEY)).toBeNull())
    expect(result.current.input).toBe('')
  })

  it('利用者の ID が無い形式（前の版で取っておいたもの）は戻さずに消す', async () => {
    sessionStorage.setItem(CHAT_UNSENT_DRAFT_KEY, '前の版の文章')
    const { result } = renderHook(() => useMuiChat())
    await waitFor(() => expect(result.current.mounted).toBe(true))
    await waitFor(() => expect(sessionStorage.getItem(CHAT_UNSENT_DRAFT_KEY)).toBeNull())
    expect(result.current.input).toBe('')
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
