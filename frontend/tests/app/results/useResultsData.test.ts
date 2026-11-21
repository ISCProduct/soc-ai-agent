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

// #2220: 推薦が0件はエラーではない。error ではなく emptyReason に分ける。
describe('useResultsData 推薦0件とエラーの区別', () => {
  beforeEach(() => {
    params = new URLSearchParams({ user_id: '1', session_id: 's1' })
    global.fetch = jest.fn().mockResolvedValue({ ok: true, json: async () => ({}) } as unknown as Response)
  })

  const respond = async (response: unknown) => {
    const { result } = renderHook(() => useResultsData())
    await act(async () => {
      recommendationResolvers['s1'](response)
    })
    return result
  }

  it('recommendations: [] は error にせず、理由を emptyReason に入れる', async () => {
    const result = await respond({
      ok: true,
      json: async () => ({ recommendations: [], reason: 'insufficient_user_scores' }),
    })
    expect(result.current.error).toBeNull()
    expect(result.current.emptyReason).toBe('insufficient_user_scores')
    expect(result.current.loading).toBe(false)
  })

  it('理由が無い0件は matching_results_empty として扱う', async () => {
    const result = await respond({ ok: true, json: async () => ({ recommendations: [] }) })
    expect(result.current.error).toBeNull()
    expect(result.current.emptyReason).toBe('matching_results_empty')
  })

  it('推薦 API の失敗（500）は error にし、emptyReason は入れない', async () => {
    const result = await respond({ ok: false, status: 500, json: async () => ({}) })
    expect(result.current.error).toBe('企業データの取得に失敗しました')
    expect(result.current.emptyReason).toBeNull()
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

  it('応募が403で拒否されたら、応募可否を取り直して画面に反映する', async () => {
    let enabled = true
    const fetchMock = jest.fn(async (url: string) => {
      if (url === '/api/applications/availability') return { ok: true, json: async () => ({ enabled }) }
      if (url === '/api/applications') {
        enabled = false // 表示後に Backend 側で閉じられた
        return { ok: false, status: 403, json: async () => ({ error: '応募は現在受け付けていません' }) }
      }
      return { ok: true, json: async () => ({}) }
    })
    global.fetch = fetchMock as unknown as typeof fetch
    const { result } = renderHook(() => useResultsData())
    await act(async () => {})
    expect(result.current.applyEnabled).toBe(true)

    await act(async () => {
      await result.current.handleApply(event, company)
    })
    expect(result.current.applyEnabled).toBe(false)
  })
})

describe('useResultsData お気に入りの解除', () => {
  const event = { stopPropagation: jest.fn() } as unknown as Parameters<ReturnType<typeof useResultsData>['handleUnfavoriteCompany']>[0]
  const favorite = {
    id: 7,
    match_id: 20,
    category_name: '北見工業',
    score: 80,
    is_favorited: true,
  }

  beforeEach(() => {
    params = new URLSearchParams({ user_id: '1', session_id: 's1' })
  })

  it('解除に成功したら、一覧を取り直す前でもハートを戻さない', async () => {
    let favoritesCalls = 0
    global.fetch = jest.fn((url: string) => {
      if (String(url).startsWith('/api/chat/favorites?')) {
        favoritesCalls += 1
        if (favoritesCalls === 1) {
          return Promise.resolve({
            ok: true,
            json: async () => ({ recommendations: [favorite] }),
          })
        }
        return new Promise(() => {})
      }
      if (url === '/api/chat/favorites/remove') {
        return Promise.resolve({ ok: true, json: async () => ({ ok: true }) })
      }
      return Promise.resolve({ ok: true, json: async () => ({}) })
    }) as unknown as typeof fetch

    const { result, rerender } = renderHook(() => useResultsData())
    await act(async () => {})
    expect(result.current.favoriteCompanies.map((company) => company.id)).toEqual(['7'])

    await act(async () => {
      recommendationResolvers.s1({
        ok: true,
        json: async () => ({ recommendations: [{ ...favorite, is_favorited: false }] }),
      })
    })
    expect(result.current.companies[0]?.isFavorited).toBe(true)

    await act(async () => {
      await result.current.handleUnfavoriteCompany(event, result.current.companies[0])
    })

    await act(async () => {
      params = new URLSearchParams({ user_id: '1', session_id: 's2' })
      rerender()
    })
    await act(async () => {
      recommendationResolvers.s2({
        ok: true,
        json: async () => ({ recommendations: [{ ...favorite, match_id: 21, is_favorited: false }] }),
      })
    })
    expect(result.current.companies[0]?.isFavorited).toBe(false)
  })

  it('初回の取得が終わる前に解除が失敗しても、一覧を取り直す', async () => {
    let favoritesCalls = 0
    let resolveFirst: (value: unknown) => void = () => {}
    global.fetch = jest.fn((url: string) => {
      if (String(url).startsWith('/api/chat/favorites?')) {
        favoritesCalls += 1
        if (favoritesCalls === 1) {
          return new Promise((resolve) => {
            resolveFirst = resolve
          })
        }
        return Promise.resolve({
          ok: true,
          json: async () => ({ recommendations: [favorite] }),
        })
      }
      if (url === '/api/chat/favorites/remove') {
        return Promise.resolve({ ok: false, json: async () => ({}) })
      }
      return Promise.resolve({ ok: true, json: async () => ({}) })
    }) as unknown as typeof fetch

    const { result } = renderHook(() => useResultsData())
    await act(async () => {})
    expect(result.current.favoriteCompanies).toEqual([])

    const company = {
      id: '7',
      matchId: 20,
      name: '北見工業',
      industry: '',
      location: '',
      employees: '',
      description: '',
      matchScore: 80,
      tags: [],
      techStack: [],
      isFavorited: true,
    }
    await act(async () => {
      await result.current.handleUnfavoriteCompany(event, company)
    })
    expect(result.current.favoriteCompanies.map((item) => item.name)).toEqual(['北見工業'])
    expect(result.current.snackbar.message).toBe('お気に入りの更新に失敗しました')

    await act(async () => {
      resolveFirst({ ok: true, json: async () => ({ recommendations: [] }) })
    })
    expect(result.current.favoriteCompanies.map((item) => item.name)).toEqual(['北見工業'])
  })
})

// SOCAIAGENT-670: 推薦への「合う／合わない」。同じ判定をもう一度押すと取り消す。
describe('useResultsData handleFeedback', () => {
  const event = { stopPropagation: jest.fn() } as unknown as Parameters<ReturnType<typeof useResultsData>['handleFeedback']>[0]

  async function setup(ok = true) {
    params = new URLSearchParams({ user_id: '1', session_id: 'fb1' })
    const fetchMock = jest.fn(async () => ({ ok, json: async () => ({}) }))
    global.fetch = fetchMock as unknown as typeof fetch
    const hook = renderHook(() => useResultsData())
    await act(async () => {
      recommendationResolvers['fb1']({
        ok: true,
        json: async () => ({ recommendations: [{ id: 1, category_name: 'A社', match_id: 10, score: 80 }] }),
      })
    })
    return { hook, fetchMock }
  }

  const bodies = (fetchMock: jest.Mock) =>
    fetchMock.mock.calls
      .filter((c) => c[0] === '/api/chat/match-feedback')
      .map((c) => JSON.parse((c[1] as { body: string }).body))

  it('押した判定を送り、その企業に反映する。もう一度押すと none で取り消す', async () => {
    const { hook, fetchMock } = await setup()
    const company = () => hook.result.current.companies[0]

    await act(async () => {
      await hook.result.current.handleFeedback(event, company(), 'unfit')
    })
    expect(bodies(fetchMock)).toEqual([{ match_id: 10, verdict: 'unfit' }])
    expect(company().feedback).toBe('unfit')

    await act(async () => {
      await hook.result.current.handleFeedback(event, company(), 'unfit')
    })
    expect(bodies(fetchMock)[1]).toEqual({ match_id: 10, verdict: 'none' })
    expect(company().feedback).toBeUndefined()

    // 判定を切り替える（合わない → 合う）
    await act(async () => {
      await hook.result.current.handleFeedback(event, company(), 'unfit')
    })
    await act(async () => {
      await hook.result.current.handleFeedback(event, company(), 'fit')
    })
    expect(company().feedback).toBe('fit')
  })

  it('保存に失敗したら、表示は変えず、エラーを知らせる', async () => {
    const { hook } = await setup(false)
    await act(async () => {
      await hook.result.current.handleFeedback(event, hook.result.current.companies[0], 'fit')
    })
    expect(hook.result.current.companies[0].feedback).toBeUndefined()
    expect(hook.result.current.snackbar).toMatchObject({ open: true, severity: 'error' })
  })
})
