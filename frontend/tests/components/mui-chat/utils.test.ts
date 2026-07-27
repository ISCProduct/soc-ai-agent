import {
  extractChoices,
  makeMessageId,
  INITIAL_GREETING,
  clearChatSessionOnEnd,
  readStoredJobCategoryId,
  writeStoredJobCategoryId,
  shouldSendChatOnKeyDown,
  jobCategoryStorageKey,
  stripChoiceLines,
  computeProgressTotals,
  shouldAutoScrollToBottom,
  CHAT_BRAND,
  findLastAssistantQuestionMessage,
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
      setItem: (key: string, value: string) => {
        store[key] = value
      },
      removeItem: (key: string) => {
        delete store[key]
      },
      _store: store,
    }
  }

  it('sessionId 削除前に chat_cache_ と職種IDを消す', () => {
    const sessionStorage = createMemoryStorage({
      chatSessionId: 'sess-42',
      chatMessages: '[]',
      [jobCategoryStorageKey('sess-42')]: '3',
    })
    const localStorage = createMemoryStorage({
      'chat_cache_sess-42': 'cached',
      chatMessages: '[]',
      chat_session_id: 'sess-42',
    })

    clearChatSessionOnEnd({ sessionStorage, localStorage })

    expect(localStorage._store['chat_cache_sess-42']).toBeUndefined()
    expect(sessionStorage._store[jobCategoryStorageKey('sess-42')]).toBeUndefined()
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

describe('job category storage', () => {
  function createMemoryStorage(initial: Record<string, string> = {}) {
    const store = { ...initial }
    return {
      getItem: (key: string) => (key in store ? store[key] : null),
      setItem: (key: string, value: string) => {
        store[key] = value
      },
      removeItem: (key: string) => {
        delete store[key]
      },
      _store: store,
    }
  }

  it('職種IDを読み書きできる', () => {
    const storage = createMemoryStorage()
    writeStoredJobCategoryId('s1', 7, storage)
    expect(readStoredJobCategoryId('s1', storage)).toBe(7)
    writeStoredJobCategoryId('s1', 0, storage)
    expect(readStoredJobCategoryId('s1', storage)).toBe(0)
  })
})

describe('shouldSendChatOnKeyDown', () => {
  it('Enter 単独では送信しない', () => {
    expect(shouldSendChatOnKeyDown({ key: 'Enter', ctrlKey: false, metaKey: false })).toBe(false)
  })

  it('Ctrl+Enter / Meta+Enter で送信する', () => {
    expect(shouldSendChatOnKeyDown({ key: 'Enter', ctrlKey: true, metaKey: false })).toBe(true)
    expect(shouldSendChatOnKeyDown({ key: 'Enter', ctrlKey: false, metaKey: true })).toBe(true)
  })

  it('IME 変換中は送信しない', () => {
    expect(
      shouldSendChatOnKeyDown({
        key: 'Enter',
        ctrlKey: true,
        metaKey: false,
        isComposing: true,
      }),
    ).toBe(false)
  })
})

describe('stripChoiceLines', () => {
  it('選択肢行を除去して質問文を残す', () => {
    const content = [
      'どの働き方が好みですか？',
      '',
      'A) リモート中心',
      'B) オフィス中心',
      '補足です',
    ].join('\n')
    expect(stripChoiceLines(content)).toBe('どの働き方が好みですか？\n\n補足です')
  })
})

describe('computeProgressTotals', () => {
  it('フェーズの想定総数を required に使う（asked ではない）', () => {
    const totals = computeProgressTotals({
      phases: [
        { valid_answers: 2, questions_asked: 2, min_questions: 3, max_questions: 5 },
        { valid_answers: 0, questions_asked: 0, min_questions: 2, max_questions: 4 },
      ],
      questionCount: 2,
      totalQuestions: 15,
    })
    expect(totals.required).toBe(9)
    expect(totals.valid).toBe(2)
    expect(totals.percent).toBe(Math.round((2 / 9) * 100))
  })

  it('フェーズ無しなら totalQuestions ベース', () => {
    expect(
      computeProgressTotals({ phases: null, questionCount: 3, totalQuestions: 15 }),
    ).toEqual({ valid: 3, required: 15, percent: 20 })
  })
})

describe('shouldAutoScrollToBottom', () => {
  it('下部付近なら true', () => {
    expect(
      shouldAutoScrollToBottom({
        scrollHeight: 1000,
        scrollTop: 900,
        clientHeight: 100,
        thresholdPx: 120,
      }),
    ).toBe(true)
  })

  it('上部を見ているときは false', () => {
    expect(
      shouldAutoScrollToBottom({
        scrollHeight: 1000,
        scrollTop: 0,
        clientHeight: 100,
        thresholdPx: 120,
      }),
    ).toBe(false)
  })
})

describe('CHAT_BRAND', () => {
  it('プライマリ青である', () => {
    expect(CHAT_BRAND).toBe('#1976d2')
  })
})

describe('findLastAssistantQuestionMessage', () => {
  it('警告メッセージを飛ばして直前の質問を返す', () => {
    const messages = [
      { role: 'assistant', content: 'A) はい\nB) いいえ' },
      { role: 'user', content: 'typo' },
      {
        role: 'assistant',
        content: '書かれた内容にはお答えできません。質問に回答してください。（1/3回目の警告）',
      },
    ]
    expect(findLastAssistantQuestionMessage(messages)?.content).toBe('A) はい\nB) いいえ')
  })
})
