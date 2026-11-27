/** ユーザー向けエラー。ALB/nginx の HTML や生ステータスは出さない */

export const GATEWAY_USER_MESSAGE =
  'ただいま接続できません。しばらくしてから再試行してください。'

export class UserFacingApiError extends Error {
  readonly status: number
  readonly gateway: boolean

  constructor(message: string, status: number) {
    super(message)
    this.name = 'UserFacingApiError'
    this.status = status
    this.gateway = isGatewayStatus(status)
  }
}

export function isGatewayStatus(status: number): boolean {
  return status === 502 || status === 503 || status === 504
}

export function looksLikeHtml(raw: string): boolean {
  const t = raw.trimStart().slice(0, 200).toLowerCase()
  return t.startsWith('<!doctype') || t.startsWith('<html') || t.includes('<head>') || t.includes('<title>502')
}

export function userFacingApiMessage(status: number, raw = ''): string {
  if (isGatewayStatus(status) || looksLikeHtml(raw)) {
    return GATEWAY_USER_MESSAGE
  }
  if (status === 401 || status === 403) {
    return 'ログインの有効期限が切れました。再度ログインしてください。'
  }
  return '処理に失敗しました。しばらくしてから再試行してください。'
}

/**
 * 接続エラーの画面（/error）へのパス。from を渡すと、エラー画面の「再試行」でそのページへ戻る。
 * code は画面には出さない（記録・調査用。#2213）。
 */
export function gatewayErrorPath(status = 502, from?: string): string {
  const code = isGatewayStatus(status) ? status : 502
  const back = safeReturnPath(from)
  return back ? `/error?code=${code}&from=${encodeURIComponent(back)}` : `/error?code=${code}`
}

/**
 * エラー画面の「再試行」で戻ってよいパスなら返す。同じサイトの中のパス（`/` で始まり、`//` や `/\` で始まらない）だけを通し、
 * エラー画面そのもの（/error）は通さない（読み直すだけになるため）。外のサイトへ飛ばされないようにする。
 */
export function safeReturnPath(from: string | null | undefined): string | null {
  if (typeof from !== 'string' || from.length === 0 || from.length > 2048) return null
  if (!from.startsWith('/') || from.startsWith('//') || from.startsWith('/\\')) return null
  for (let i = 0; i < from.length; i++) {
    const c = from.charCodeAt(i)
    if (c < 0x20 || c === 0x7f) return null // 改行・タブなどの制御文字
  }
  const path = from.split(/[?#]/)[0]
  if (path === '/error' || path.startsWith('/error/')) return null
  return from
}
