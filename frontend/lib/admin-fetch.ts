/**
 * 管理画面クライアント側の fetch + JSON パース。
 *
 * 管理画面の各ハンドラは `fetch` を素で呼んでおり、通信断で reject すると
 * `setLoading(false)` に到達せずボタンが disabled のまま固着していた (#1066)。
 * 失敗を必ず「日本語メッセージを持つ例外」に正規化して throw することで、
 * 呼び出し側は try/catch/finally だけで済むようにする。
 *
 * 状態管理は既存流儀のまま各コンポーネントの useState + <ErrorAlert /> に任せる。
 */
import { fetchWithTimeout, FetchTimeoutError } from './fetch-timeout'
import { UserFacingApiError, looksLikeHtml, userFacingApiMessage } from './user-facing-error'

/** 原因を特定できない例外に出す既定文言。英語の生メッセージを画面に出さないため */
const UNKNOWN_ERROR_MESSAGE = '通信エラーが発生しました。しばらくしてから再試行してください。'

/** サーバーが返した JSON から表示可能なエラー文言を取り出す。無ければ空文字列 */
function serverErrorMessage(raw: string): string {
  if (!raw.trim() || looksLikeHtml(raw)) return ''
  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === 'object') {
      const { error, message } = parsed as { error?: unknown; message?: unknown }
      if (typeof error === 'string' && error.trim()) return error
      if (typeof message === 'string' && message.trim()) return message
    }
  } catch {
    // JSON でないボディは表示に使わない（ALB の HTML 等が混ざるため）
  }
  return ''
}

/**
 * 管理APIを呼び出して JSON を返す。失敗時は必ず throw する。
 *
 * `fallbackMessage` はサーバーがエラー文言を返さなかった場合の表示文言
 * （例: '権限更新に失敗しました'）。502/503/504 や HTML エラーページの場合は
 * fallback より優先してゲートウェイ向けの文言を出す。
 */
export async function adminFetchJson<T>(
  input: RequestInfo | URL,
  init: RequestInit | undefined,
  fallbackMessage: string,
): Promise<T> {
  const res = await fetchWithTimeout(input, init)
  // res.json() は ALB/nginx の HTML エラーページで throw するため text で受ける
  const raw = await res.text()

  if (!res.ok) {
    const gatewayLike = looksLikeHtml(raw) || res.status === 502 || res.status === 503 || res.status === 504
    const message = gatewayLike
      ? userFacingApiMessage(res.status, raw)
      : serverErrorMessage(raw) || fallbackMessage
    throw new UserFacingApiError(message, res.status)
  }

  if (!raw.trim()) return undefined as T
  try {
    return JSON.parse(raw) as T
  } catch {
    throw new UserFacingApiError(fallbackMessage, res.status)
  }
}

/**
 * catch 節で受けた値を、そのまま画面に出せる日本語メッセージへ変換する。
 * UserFacingApiError / FetchTimeoutError は日本語文言を持っているのでそのまま使う。
 */
export function toAdminErrorMessage(error: unknown): string {
  if (error instanceof UserFacingApiError || error instanceof FetchTimeoutError) {
    return error.message
  }
  return UNKNOWN_ERROR_MESSAGE
}
