/**
 * @jest-environment jsdom
 */
import { TextDecoder, TextEncoder } from 'util'
import { act, renderHook } from '@testing-library/react'
import { useResumePage } from '@/app/resume/hooks/useResumePage'

// jsdom には TextEncoder / TextDecoder が無い（ストリームの解析で使う）
Object.assign(global, { TextEncoder, TextDecoder })

jest.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(),
}))

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({}),
    getStoredUser: () => ({ user_id: 'u1' }),
  },
}))

describe('useResumePage handleUpload (#948)', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
  })

  it('再アップロード時に直前のレビュー関連stateを全てリセットする', async () => {
    const { result } = renderHook(() => useResumePage())

    // #1055 でアップロード前バリデーションが入ったため、送信元を設定しておく
    act(() => {
      result.current.setSourceUrl('https://example.com/resume.pdf')
    })

    await act(async () => {
      await result.current.handleReview()
    })
    expect(result.current.reviewError).not.toBe('')

    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ document: { id: 1 } }),
    } as unknown as Response)

    await act(async () => {
      await result.current.handleUpload()
    })

    expect(result.current.reviewError).toBe('')
    expect(result.current.reviewLoading).toBe(false)
    expect(result.current.annotateError).toBe('')
    expect(result.current.ragReport).toBe('')
    expect(result.current.scoresBefore).toBeNull()
    expect(result.current.scoresAfter).toBeNull()
    expect(result.current.documentId).toBe(1)

    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      text: async () => 'upload failed',
    } as unknown as Response)

    await act(async () => {
      await result.current.handleUpload()
    })
    expect(result.current.documentId).toBeNull()
  })
})

describe('useResumePage handleUpload バリデーション (#1055)', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
  })

  it('ファイルもURLも未設定なら送信せずメッセージを表示する', async () => {
    const fetchMock = jest.fn()
    global.fetch = fetchMock as unknown as typeof fetch
    const { result } = renderHook(() => useResumePage())

    await act(async () => {
      await result.current.handleUpload()
    })

    expect(fetchMock).not.toHaveBeenCalled()
    expect(result.current.uploadError).toBe('ファイルを選択するかURLを入力してください')
    expect(result.current.loading).toBe(false)
  })

  it('URLが空白のみでも未入力として扱う', async () => {
    const fetchMock = jest.fn()
    global.fetch = fetchMock as unknown as typeof fetch
    const { result } = renderHook(() => useResumePage())

    act(() => {
      result.current.setSourceUrl('   ')
    })
    await act(async () => {
      await result.current.handleUpload()
    })

    expect(fetchMock).not.toHaveBeenCalled()
    expect(result.current.uploadError).toBe('ファイルを選択するかURLを入力してください')
  })

  it('バリデーションで弾いたときは既存のレビュー結果を消さない', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ document: { id: 42 } }),
    } as unknown as Response)
    const { result } = renderHook(() => useResumePage())

    act(() => {
      result.current.setSourceUrl('https://example.com/resume.pdf')
    })
    await act(async () => {
      await result.current.handleUpload()
    })
    expect(result.current.documentId).toBe(42)

    // 送信元を消してから再度アップロードを試みる
    act(() => {
      result.current.setSourceUrl('')
    })
    await act(async () => {
      await result.current.handleUpload()
    })

    expect(result.current.uploadError).toBe('ファイルを選択するかURLを入力してください')
    expect(result.current.documentId).toBe(42) // 直前の結果が残っている
  })

  it('URLが設定されていれば送信する', async () => {
    const fetchMock = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ document: { id: 7 } }),
    } as unknown as Response)
    global.fetch = fetchMock as unknown as typeof fetch
    const { result } = renderHook(() => useResumePage())

    act(() => {
      result.current.setSourceUrl('https://example.com/resume.pdf')
    })
    await act(async () => {
      await result.current.handleUpload()
    })

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(result.current.documentId).toBe(7)
  })

  it('ファイルが選択されていれば送信する', async () => {
    const fetchMock = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ document: { id: 8 } }),
    } as unknown as Response)
    global.fetch = fetchMock as unknown as typeof fetch
    const { result } = renderHook(() => useResumePage())

    act(() => {
      result.current.setFile(new File(['x'], 'resume.pdf', { type: 'application/pdf' }))
    })
    await act(async () => {
      await result.current.handleUpload()
    })

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(result.current.documentId).toBe(8)
  })
})

describe('useResumePage handleReview のストリーム中断 (SOCAIAGENT-664)', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
  })

  // jsdom には Response / ReadableStream が無いので、必要な形だけの応答を作る
  const sse = (...events: object[]) => {
    const chunks = events.map((e) => new TextEncoder().encode(`data: ${JSON.stringify(e)}\n\n`))
    return {
      ok: true,
      body: {
        getReader: () => ({
          read: async () => (chunks.length ? { done: false, value: chunks.shift() } : { done: true, value: undefined }),
          cancel: async () => undefined,
        }),
      },
    } as unknown as Response
  }
  const json = (body: unknown) => ({ ok: true, json: async () => body }) as unknown as Response

  async function setup(reviewResponse: () => Response) {
    global.fetch = jest.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/resume/upload')) return json({ document: { id: 1 } })
      if (url.includes('/api/resume/review/stream')) return reviewResponse()
      return json({})
    }) as unknown as typeof fetch

    const { result } = renderHook(() => useResumePage())
    // user_id は副作用で入るので、反映を待つ
    await act(async () => {})
    act(() => {
      result.current.setSourceUrl('https://example.com/resume.pdf')
      result.current.setJobTitle('エンジニア')
    })
    await act(async () => {
      await result.current.handleUpload()
    })
    return result
  }

  it('complete も error も受け取らずに閉じたら中断として扱う', async () => {
    const result = await setup(() => sse({ type: 'chunk', text: '途中まで' }))
    await act(async () => {
      await result.current.handleReview()
    })
    expect(result.current.reviewInterrupted).toBe(true)
    expect(result.current.review).toBeNull()
    expect(result.current.reviewLoading).toBe(false)
  })

  it('complete を受け取ったら中断にしない', async () => {
    const result = await setup(() =>
      sse({ type: 'chunk', text: 'a' }, { type: 'complete', review: { id: 1 }, items: [], annotated_available: true }),
    )
    await act(async () => {
      await result.current.handleReview()
    })
    expect(result.current.reviewInterrupted).toBe(false)
    expect(result.current.review).not.toBeNull()
  })

  it('error を受け取ったら上流の文言は出さず固定文言にし、中断表示にはしない', async () => {
    const result = await setup(() => sse({ type: 'error', message: 'openai.RateLimitError: sk-secret' }))
    await act(async () => {
      await result.current.handleReview()
    })
    expect(result.current.reviewError).toBe('レビューに失敗しました。時間をおいてもう一度お試しください。')
    expect(result.current.reviewError).not.toContain('sk-secret')
    expect(result.current.reviewInterrupted).toBe(false)
  })

  it('再試行を始めたら中断表示を消す', async () => {
    let first = true
    const result = await setup(() => {
      if (first) {
        first = false
        return sse({ type: 'chunk', text: 'x' })
      }
      return sse({ type: 'complete', review: { id: 1 }, items: [], annotated_available: true })
    })
    await act(async () => {
      await result.current.handleReview()
    })
    expect(result.current.reviewInterrupted).toBe(true)
    await act(async () => {
      await result.current.handleReview()
    })
    expect(result.current.reviewInterrupted).toBe(false)
    expect(result.current.review).not.toBeNull()
  })
})
