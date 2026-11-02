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

// SOCAIAGENT-403: 応募を閉じている間は、画面側でも応募APIを呼ばない。
describe('useResultsData handleApply と応募の開閉', () => {
  const company = {
    id: '1',
    matchId: 10,
    name: 'A社',
    industry: '',
    location: '',
    employees: '',
    description: '',
    matchScore: 90,
    tags: [],
    techStack: [],
    isFavorited: false,
    isApplied: false,
  } as unknown as Parameters<ReturnType<typeof useResultsData>['handleApply']>[1]
  const event = { stopPropagation: jest.fn() } as unknown as Parameters<ReturnType<typeof useResultsData>['handleApply']>[0]

  function mockFetch(enabled: boolean) {
    const fetchMock = jest.fn(async (url: string) => {
      if (url === '/api/applications/availability') return { ok: true, json: async () => ({ enabled }) }
      if (url === '/api/applications') return { ok: true, json: async () => ({ id: 99 }) }
      return { ok: true, json: async () => ({}) }
    })
    global.fetch = fetchMock as unknown as typeof fetch
    return fetchMock
  }
  const applyCalls = (m: jest.Mock) => m.mock.calls.filter(([u]) => u === '/api/applications')

  beforeEach(() => {
    params = new URLSearchParams({ user_id: '1', session_id: 's1' })
  })

  it('閉じている間は handleApply しても POST /api/applications を呼ばない', async () => {
    const fetchMock = mockFetch(false)
    const { result } = renderHook(() => useResultsData())
    await act(async () => {})
    expect(result.current.applyEnabled).toBe(false)

    await act(async () => {
      await result.current.handleApply(event, company)
    })
    expect(applyCalls(fetchMock)).toHaveLength(0)
  })

  it('開いていれば handleApply が POST /api/applications を呼ぶ', async () => {
    const fetchMock = mockFetch(true)
    const { result } = renderHook(() => useResultsData())
    await act(async () => {})
    expect(result.current.applyEnabled).toBe(true)

    await act(async () => {
      await result.current.handleApply(event, company)
    })
    expect(applyCalls(fetchMock)).toHaveLength(1)
  })
})
