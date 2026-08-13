/** 会場Wi-Fi想定。一覧系 GET が半開きのまま固まらないようにする */

export const LIST_FETCH_TIMEOUT_MS = 15_000

export class FetchTimeoutError extends Error {
  constructor(message = '通信がタイムアウトしました。再試行してください。') {
    super(message)
    this.name = 'FetchTimeoutError'
  }
}

export async function fetchWithTimeout(
  input: RequestInfo | URL,
  init?: RequestInit,
  timeoutMs = LIST_FETCH_TIMEOUT_MS,
): Promise<Response> {
  const ctrl = new AbortController()
  const onAbort = () => ctrl.abort()
  const timer = setTimeout(onAbort, timeoutMs)
  init?.signal?.addEventListener('abort', onAbort, { once: true })
  try {
    return await fetch(input, { ...init, signal: ctrl.signal })
  } catch (e) {
    if (ctrl.signal.aborted && init?.signal?.aborted !== true) {
      throw new FetchTimeoutError()
    }
    throw e
  } finally {
    clearTimeout(timer)
    init?.signal?.removeEventListener('abort', onAbort)
  }
}
