import {
  extractChoices,
  makeMessageId,
  INITIAL_GREETING,
  clearChatSessionOnEnd,
} from '@/components/mui-chat/utils'

describe('extractChoices', () => {
  it('A) 形式の選択肢を抽出する', () => {
    const content = [
      'どの働き方が好みですか？',
      '',
      'A) リモート中心',
      'B) オフィス中心',
      'C) ハイブリッド',
    ].join('\n')

    expect(extractChoices(content)).toEqual([
      { value: 'A', label: 'A', text: 'リモート中心' },
      { value: 'B', label: 'B', text: 'オフィス中心' },
      { value: 'C', label: 'C', text: 'ハイブリッド' },
    ])
  })

  it('1) / 1. 形式の選択肢を抽出する', () => {
    const content = ['質問文', '1) 新しい技術', '2. 設計', '3．人と関わる'].join('\n')

    expect(extractChoices(content)).toEqual([
      { value: '1', label: '1', text: '新しい技術' },
      { value: '2', label: '2', text: '設計' },
      { value: '3', label: '3', text: '人と関わる' },
    ])
  })

  it('A： / A、 / A. 形式も抽出する', () => {
    const content = ['A：主導する', 'B、サポートする', 'C. 状況次第'].join('\n')

    expect(extractChoices(content)).toEqual([
      { value: 'A', label: 'A', text: '主導する' },
      { value: 'B', label: 'B', text: 'サポートする' },
      { value: 'C', label: 'C', text: '状況次第' },
    ])
  })

  it('空行はスキップし、選択肢以外の行は無視する', () => {
    const content = [
      '説明文です。',
      '',
      '   ',
      'A) 選択肢A',
      '補足コメント',
      'B) 選択肢B',
      '',
    ].join('\n')

    expect(extractChoices(content)).toEqual([
      { value: 'A', label: 'A', text: '選択肢A' },
      { value: 'B', label: 'B', text: '選択肢B' },
    ])
  })

  it('選択肢が無い場合は空配列を返す', () => {
    expect(extractChoices('こんにちは！\n\n質問です。')).toEqual([])
    expect(extractChoices('')).toEqual([])
  })
})

describe('makeMessageId / INITIAL_GREETING', () => {
  it('makeMessageId は一意な文字列を返す', () => {
    const a = makeMessageId()
    const b = makeMessageId()
    expect(a).not.toBe(b)
    expect(a.length).toBeGreaterThan(0)
  })

  it('INITIAL_GREETING は挨拶文を含む', () => {
    expect(INITIAL_GREETING).toContain('キャリアエージェント')
  })
})

describe('clearChatSessionOnEnd', () => {
  function createMemoryStorage(initial: Record<string, string> = {}) {
    const store = { ...initial }
    return {
      getItem: (key: string) => (key in store ? store[key] : null),
      removeItem: (key: string) => {
        delete store[key]
      },
      _store: store,
    }
  }

  it('sessionId 削除前に chat_cache_ を消す', () => {
    const sessionStorage = createMemoryStorage({
      chatSessionId: 'sess-42',
      chatMessages: '[]',
    })
    const localStorage = createMemoryStorage({
      'chat_cache_sess-42': 'cached',
      chatMessages: '[]',
      chat_session_id: 'sess-42',
    })

    clearChatSessionOnEnd({ sessionStorage, localStorage })

    expect(localStorage._store['chat_cache_sess-42']).toBeUndefined()
    expect(sessionStorage._store.chatSessionId).toBeUndefined()
    expect(sessionStorage._store.chatMessages).toBeUndefined()
    expect(localStorage._store.chatMessages).toBeUndefined()
    expect(localStorage._store.chat_session_id).toBeUndefined()
  })

  it('sessionId が無い場合も他キーは削除する', () => {
    const sessionStorage = createMemoryStorage({ chatMessages: '[]' })
    const localStorage = createMemoryStorage({
      chatMessages: '[]',
      chat_session_id: 'x',
    })

    clearChatSessionOnEnd({ sessionStorage, localStorage })

    expect(sessionStorage._store.chatMessages).toBeUndefined()
    expect(localStorage._store.chat_session_id).toBeUndefined()
  })
})
