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

/** チャット本文が上限を超えたときにサーバーが返す文言（SOCAIAGENT-702）。原因なのでそのまま見せる */
export const CHAT_TOO_LONG_MESSAGE = '文字数が上限を超えています'

export function userFacingApiMessage(status: number, raw = ''): string {
  if (isGatewayStatus(status) || looksLikeHtml(raw)) {
    return GATEWAY_USER_MESSAGE
  }
  if (status === 400 && raw.includes(CHAT_TOO_LONG_MESSAGE)) {
    return CHAT_TOO_LONG_MESSAGE
  }
  if (status === 401 || status === 403) {
    return 'ログインの有効期限が切れました。再度ログインしてください。'
  }
  return '処理に失敗しました。しばらくしてから再試行してください。'
}

export function gatewayErrorPath(status = 502): string {
  const code = isGatewayStatus(status) ? status : 502
  return `/error?code=${code}`
}
