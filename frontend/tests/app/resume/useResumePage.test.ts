/**
 * @jest-environment jsdom
 */
import { act, renderHook } from '@testing-library/react'
import { useResumePage } from '@/app/resume/hooks/useResumePage'

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
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('再アップロード時に直前のレビュー関連stateを全てリセットする', async () => {
    const { result } = renderHook(() => useResumePage())

    // documentId未設定でhandleReviewを呼び、reviewErrorを非空にしておく
    await act(async () => {
      await result.current.handleReview()
    })
    expect(result.current.reviewError).not.toBe('')

    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ document: { id: 1 } }),
    })

    await act(async () => {
      await result.current.handleUpload()
    })

    expect(result.current.reviewError).toBe('')
    expect(result.current.reviewLoading).toBe(false)
    expect(result.current.annotateError).toBe('')
    expect(result.current.ragReport).toBe('')
    expect(result.current.scoresBefore).toBeNull()
    expect(result.current.scoresAfter).toBeNull()
  })
})
