import { FetchTimeoutError, fetchAndReadWithTimeout } from '@/lib/fetch-timeout'

/** 面接の企業WEB検索。バックエンドの40秒より少し長く、応答の本文を受け取れる。 */
export const WEB_SEARCH_TIMEOUT_MS = 45_000

const TOO_MANY_MESSAGE = '検索の回数が多すぎます。少し時間を置いてからお試しください。'
const FAILED_MESSAGE = '検索できませんでした。しばらくしてからもう一度お試しください。'

export type WebSearchCompany = { name: string; description: string }

/** 学生に見せる検索失敗。ブラウザ由来の英語メッセージは入れない。 */
export class WebSearchError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'WebSearchError'
  }
}

export async function fetchWebSearchCompanies(
  query: string,
  signal?: AbortSignal,
): Promise<WebSearchCompany[]> {
  try {
    const data = await fetchAndReadWithTimeout(
      `/api/companies/web-search?q=${encodeURIComponent(query)}`,
      { cache: 'no-store', signal },
      WEB_SEARCH_TIMEOUT_MS,
      async (r) => {
        if (!r.ok) {
          throw new WebSearchError(r.status === 429 ? TOO_MANY_MESSAGE : FAILED_MESSAGE)
        }
        return r.json() as Promise<{ results?: WebSearchCompany[] }>
      },
    )
    return Array.isArray(data?.results) ? data.results : []
  } catch (e) {
    if (e instanceof WebSearchError || e instanceof FetchTimeoutError) throw e
    if (e instanceof Error && e.name === 'AbortError') throw e
    throw new WebSearchError(FAILED_MESSAGE)
  }
}
