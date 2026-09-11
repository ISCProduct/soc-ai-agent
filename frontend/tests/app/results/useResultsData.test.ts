/**
 * @jest-environment jsdom
 */
import { act, renderHook } from '@testing-library/react'
import { useResultsData } from '@/app/results/hooks/useResultsData'

let params = new URLSearchParams({ user_id: '1', session_id: 's1' })

jest.mock('next/navigation', () => ({
  useSearchParams: () => params,
  useRouter: () => ({ replace: jest.fn(), push: jest.fn() }),
}))

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({}),
    getStoredUser: () => ({ is_guest: false }),
  },
}))

// #988: fetchAnalysis(/api/chat/analysis)は別経路(global.fetch)、
// fetchCompanies(/api/chat/recommendations)はfetchWithTimeout経由なので、
// 後者だけを制御して古いレスポンスが新しいレスポンスを上書きしないことを確認する。
const recommendationResolvers: Record<string, (value: unknown) => void> = {}
jest.mock('@/lib/fetch-timeout', () => ({
  fetchWithTimeout: jest.fn((url: string) => {
    const sessionId = new URL(url, 'http://localhost').searchParams.get('session_id') ?? ''
    return new Promise((resolve) => {
      recommendationResolvers[sessionId] = resolve
    })
  }),
}))

describe('useResultsData fetchCompanies race condition (#988)', () => {
  beforeEach(() => {
    params = new URLSearchParams({ user_id: '1', session_id: 's1' })
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({}),
    } as unknown as Response)
  })

  it('古いsession_idのレスポンスが後から届いても、新しいセッションの表示を上書きしない', async () => {
    const { result, rerender } = renderHook(() => useResultsData())

    // s1のリクエストが飛んだ状態でsession_idをs2に切り替える
    await act(async () => {
      params = new URLSearchParams({ user_id: '1', session_id: 's2' })
      rerender()
    })

    // s2のレスポンスが先に解決する
    await act(async () => {
      recommendationResolvers['s2']({
        ok: true,
        json: async () => ({
          recommendations: [{ id: 2, name: 'S2 Company', match_id: 2 }],
        }),
      })
    })

    expect(result.current.companies.map((c) => c.name)).toEqual(['S2 Company'])

    // 古いs1のレスポンスが遅れて届く
    await act(async () => {
      recommendationResolvers['s1']({
        ok: true,
        json: async () => ({
          recommendations: [{ id: 1, name: 'S1 Company (stale)', match_id: 1 }],
        }),
      })
    })

    // s2の表示が古いs1のデータで上書きされていないこと
    expect(result.current.companies.map((c) => c.name)).toEqual(['S2 Company'])
  })
})
