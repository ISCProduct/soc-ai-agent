/** @jest-environment jsdom */

import { FetchTimeoutError } from '@/lib/fetch-timeout'
import { fetchWebSearchCompanies, WEB_SEARCH_TIMEOUT_MS, WebSearchError } from '@/lib/interview/web-search'

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response
}

describe('面接の企業WEB検索', () => {
  const original = global.fetch

  afterEach(() => {
    global.fetch = original
    jest.useRealTimers()
  })

  it('429 は回数の案内にする', async () => {
    global.fetch = jest.fn().mockResolvedValue(jsonResponse(429, { error: 'raw' }))
    await expect(fetchWebSearchCompanies('架空電機')).rejects.toEqual(expect.any(WebSearchError))
    await expect(fetchWebSearchCompanies('架空電機')).rejects.toThrow(
      '検索の回数が多すぎます。少し時間を置いてからお試しください。',
    )
  })

  it('500 は再試行の案内にし、本文は使わない', async () => {
    global.fetch = jest.fn().mockResolvedValue(jsonResponse(500, { error: 'SECRET_UPSTREAM_BODY' }))
    await expect(fetchWebSearchCompanies('架空電機')).rejects.toThrow(
      '検索できませんでした。しばらくしてからもう一度お試しください。',
    )
  })

  it('候補なしは空配列で、失敗ではない', async () => {
    global.fetch = jest.fn().mockResolvedValue(jsonResponse(200, { results: [] }))
    await expect(fetchWebSearchCompanies('架空電機')).resolves.toEqual([])
  })

  it('通信そのものの失敗は英語のまま出さない', async () => {
    global.fetch = jest.fn().mockRejectedValue(new TypeError('Failed to fetch'))
    await expect(fetchWebSearchCompanies('架空電機')).rejects.toThrow(
      '検索できませんでした。しばらくしてからもう一度お試しください。',
    )
  })

  it('45秒で打ち切る', async () => {
    jest.useFakeTimers()
    global.fetch = jest.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      return new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () =>
          reject(Object.assign(new Error('aborted'), { name: 'AbortError' })),
        )
      })
    }) as unknown as typeof fetch

    const settled = fetchWebSearchCompanies('架空電機').then(
      () => 'resolved',
      (e: unknown) => e,
    )
    await jest.advanceTimersByTimeAsync(WEB_SEARCH_TIMEOUT_MS)
    await expect(settled).resolves.toBeInstanceOf(FetchTimeoutError)
  })

  it('呼び出し元が中断したらタイムアウト扱いにしない', async () => {
    const ac = new AbortController()
    global.fetch = jest.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      return new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () =>
          reject(Object.assign(new Error('aborted'), { name: 'AbortError' })),
        )
      })
    }) as unknown as typeof fetch

    const pending = fetchWebSearchCompanies('架空電機', ac.signal)
    ac.abort()
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
  })
})
