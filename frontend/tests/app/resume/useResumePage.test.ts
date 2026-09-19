/**
 * @jest-environment jsdom
 */
import { act, renderHook } from '@testing-library/react'
import { useResumePage } from '@/app/resume/hooks/useResumePage'
import { TextDecoder as NodeTextDecoder, TextEncoder as NodeTextEncoder } from 'util'

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

// #1332: 保存に失敗してもレビューは画面に出る。save_error を握りつぶすと
// 学生は「保存済み」と思って閉じ、結果が失われる（不具合の症状がそのまま再現する）
describe('useResumePage SSE save_error (#1332)', () => {
  const originalFetch = global.fetch

  // jsdom には TextEncoder/TextDecoder が無い。フック側が TextDecoder を使うので補う
  beforeAll(() => {
    const g = globalThis as unknown as { TextEncoder?: unknown; TextDecoder?: unknown }
    g.TextEncoder = g.TextEncoder ?? NodeTextEncoder
    g.TextDecoder = g.TextDecoder ?? NodeTextDecoder
  })

  afterEach(() => {
    global.fetch = originalFetch
  })

  const streamResponse = (events: object[]) => {
    const body = events.map((e) => `data: ${JSON.stringify(e)}\n`).join('')
    const chunk = new NodeTextEncoder().encode(body)
    let sent = false
    return {
      ok: true,
      body: {
        getReader: () => ({
          read: async () => {
            if (sent) return { done: true, value: undefined }
            sent = true
            return { done: false, value: chunk }
          },
          cancel: async () => undefined,
        }),
      },
    } as unknown as Response
  }

  const review = { score: 80, summary: 'ok' }

  it('save_error を受け取ったら画面に出すための state に残す', async () => {
    global.fetch = jest.fn().mockImplementation(async (url: string) => {
      if (String(url).includes('/api/resume/review/stream')) {
        return streamResponse([
          { type: 'save_error', message: '保存に失敗しました' },
          { type: 'complete', review, items: [], annotated_available: false },
        ])
      }
      return { ok: true, json: async () => ({ document: { id: 3 } }) } as unknown as Response
    })

    const { result } = renderHook(() => useResumePage())
    act(() => {
      result.current.setSourceUrl('https://example.com/resume.pdf')
      result.current.setJobTitle('バックエンドエンジニア')
    })
    await act(async () => {
      await result.current.handleUpload()
    })
    await act(async () => {
      await result.current.handleReview()
    })

    expect(result.current.saveError).toBe('保存に失敗しました')
    expect(result.current.review).not.toBeNull()
  })

  it('保存が成功した通常のレビューでは save_error は空のまま', async () => {
    global.fetch = jest.fn().mockImplementation(async (url: string) => {
      if (String(url).includes('/api/resume/review/stream')) {
        return streamResponse([{ type: 'complete', review, items: [], annotated_available: false }])
      }
      return { ok: true, json: async () => ({ document: { id: 4 } }) } as unknown as Response
    })

    const { result } = renderHook(() => useResumePage())
    act(() => {
      result.current.setSourceUrl('https://example.com/resume.pdf')
      result.current.setJobTitle('バックエンドエンジニア')
    })
    await act(async () => {
      await result.current.handleUpload()
    })
    await act(async () => {
      await result.current.handleReview()
    })

    expect(result.current.saveError).toBe('')
    expect(result.current.review).not.toBeNull()
  })
})
