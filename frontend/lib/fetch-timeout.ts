/** 会場Wi-Fi想定。一覧系 GET が半開きのまま固まらないようにする */

export const LIST_FETCH_TIMEOUT_MS = 15_000

export class FetchTimeoutError extends Error {
  constructor(message = '通信がタイムアウトしました。再試行してください。') {
    super(message)
    this.name = 'FetchTimeoutError'
  }
}

/**
 * ヘッダー受信だけでなく、本文の読み込み（read）が終わるまでタイムアウトを効かせる（#1476）。
 *
 * fetchWithTimeout はヘッダーを受け取った時点でタイマーを解除するため、その後の
 * `res.arrayBuffer()` / `res.text()` が半開きのまま固まると永久に解決しない。
 * ここでは fetch に渡した AbortController を read の完了まで保持するので、
 * 本文の受信中に止まっても timeoutMs で中断され、呼び出し元は必ず戻る。
 */
export async function fetchAndReadWithTimeout<T>(
  input: RequestInfo | URL,
  init: RequestInit | undefined,
  timeoutMs: number,
  read: (res: Response) => Promise<T>,
): Promise<T> {
  const ctrl = new AbortController()
  const onAbort = () => ctrl.abort()
  const timer = setTimeout(onAbort, timeoutMs)
  init?.signal?.addEventListener('abort', onAbort, { once: true })
  try {
    return await read(await fetch(input, { ...init, signal: ctrl.signal }))
  } catch (e) {
    // 本文の受信中に中断した場合は read 側から AbortError が飛ぶ。文言をここで揃える。
    if (ctrl.signal.aborted && init?.signal?.aborted !== true) {
      throw new FetchTimeoutError()
    }
    throw e
  } finally {
    clearTimeout(timer)
    init?.signal?.removeEventListener('abort', onAbort)
  }
}

/**
 * ヘッダー受信までのタイムアウト付き fetch。
 * 本文の読み込みは対象外なので、大きな本文を読む呼び出しは
 * fetchAndReadWithTimeout を使うこと（#1476）。
 */
export function fetchWithTimeout(
  input: RequestInfo | URL,
  init?: RequestInit,
  timeoutMs = LIST_FETCH_TIMEOUT_MS,
): Promise<Response> {
  // read は素通し。返した時点で finally がタイマーを解除するので、
  // 本文の読み込み中は中断されない（従来どおりの挙動）。
  return fetchAndReadWithTimeout(input, init, timeoutMs, async res => res)
}
