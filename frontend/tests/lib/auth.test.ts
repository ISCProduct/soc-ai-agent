/** @jest-environment jsdom */

import { authService, extractErrorMessage } from '@/lib/auth'

function jsonResponse(body: unknown): Response {
  return { json: async () => body } as Response
}

function createStorageMock(initial: Record<string, string> = {}): Storage {
  const store = new Map<string, string>(Object.entries(initial))
  return {
    get length() {
      return store.size
    },
    clear: () => store.clear(),
    getItem: (key: string) => store.get(key) ?? null,
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key)
    },
    setItem: (key: string, value: string) => {
      store.set(key, value)
    },
  }
}

describe('extractErrorMessage', () => {
  it('既知の英語エラーを日本語に変換する', async () => {
    const res = jsonResponse({ error: 'invalid email or password' })
    expect(await extractErrorMessage(res, 'fallback')).toBe(
      'メールアドレスまたはパスワードが正しくありません。',
    )
  })

  it('未知のエラーメッセージはフォールバックを返す(英語の生文字列をそのまま出さない)', async () => {
    const res = jsonResponse({ error: 'some unmapped backend error' })
    expect(await extractErrorMessage(res, 'フォールバック文言')).toBe('フォールバック文言')
  })

  it('JSONでないレスポンスはフォールバックを返す', async () => {
    const res = { json: async () => { throw new Error('not json') } } as unknown as Response
    expect(await extractErrorMessage(res, 'フォールバック文言')).toBe('フォールバック文言')
  })
})

describe('authService.logout', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'sessionStorage', {
      value: createStorageMock({ user: 'u', token: 't', chatSessionId: 'sess-42' }),
      configurable: true,
    })
    Object.defineProperty(window, 'localStorage', {
      value: createStorageMock({
        user: 'u',
        token: 't',
        chat_cache_sess42: 'legacy-cache',
        chat_session_id: 'sess42',
        'chat_cache_sess-42': 'cache',
        currentSessionId: 'sess-history-99',
      }),
      configurable: true,
    })
    global.fetch = jest.fn().mockResolvedValue({ ok: true } as Response)
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('稼働中のチャット機能が使うキー(chatSessionId/chat_cache_)もクリアする(#947)', () => {
    authService.logout()

    // 稼働中のチャット機能（useMuiChat）が使うキー
    expect(sessionStorage.getItem('chatSessionId')).toBeNull()
    expect(localStorage.getItem('chat_cache_sess-42')).toBeNull()
    // chat-history選択画面が一時的に載せる引き継ぎ用キーも消える(#947)
    expect(localStorage.getItem('currentSessionId')).toBeNull()

    // レガシーキーも引き続きクリアされる
    expect(localStorage.getItem('chat_cache_sess42')).toBeNull()
    expect(localStorage.getItem('chat_session_id')).toBeNull()

    // 認証情報も従来通りクリアされる
    expect(sessionStorage.getItem('user')).toBeNull()
    expect(localStorage.getItem('user')).toBeNull()
  })
})
