import type { Breadcrumb, ErrorEvent, EventHint } from '@sentry/nextjs'

const SENSITIVE_HEADERS = new Set([
  'authorization',
  'cookie',
  'x-admin-token',
  'x-user-token',
  'x-company-user-token',
  'x-internal-token',
  // CloudFront が本番の全リクエストへ必ず付ける経路証明トークン(#1407)。
  // 漏れると ALB 直叩きで CloudFront 経由と誤認させ、詐称XFFを署名させられる。
  'x-origin-token',
  // httpContextIntegration が必ず載せる。遷移元のクエリ（＝トークン）が入る
  'referer',
])

/**
 * URL からクエリ文字列とフラグメントを落とす。
 *
 * クエリにワンタイムトークンやユーザー情報を載せている画面が実在する:
 *   /verify-email?token=...            メール認証トークン
 *   /company-portal/setup?token=...    企業担当者の招待トークン
 *   /auth/callback?user=...            Base64 のユーザー情報
 * これらのページでJSエラーが1件起きるだけで、そのまま Sentry へ送られてしまう。
 */
function stripUrlSecrets(url: string): string {
  const cut = url.search(/[?#]/)
  return cut === -1 ? url : url.slice(0, cut)
}

/**
 * Sentry 送信前に認証ヘッダー・Cookie・ボディ・URLのクエリを落とす（#619 / #1185）。
 * 履歴書・チャット本文などの個人情報混入を防ぐ。
 */
export function scrubSentryEvent(event: ErrorEvent, _hint?: EventHint): ErrorEvent | null {
  if (event.request) {
    delete event.request.data
    delete event.request.cookies
    if (event.request.headers) {
      for (const key of Object.keys(event.request.headers)) {
        // Referer にも遷移元のクエリ（＝トークン）が載る
        if (SENSITIVE_HEADERS.has(key.toLowerCase())) {
          delete event.request.headers[key]
        }
      }
    }
    if (event.request.query_string) {
      event.request.query_string = ''
    }
    // httpContextIntegration が window.location.href をそのまま入れるため、
    // query_string を空にするだけでは足りない。
    if (typeof event.request.url === 'string') {
      event.request.url = stripUrlSecrets(event.request.url)
    }
  }
  return event
}

/**
 * パンくず（fetch / navigation / console）からもクエリを落とす。
 * パンくずは beforeSend の対象外なので、ここで潰さないと素通りする。
 */
export function scrubSentryBreadcrumb(breadcrumb: Breadcrumb): Breadcrumb | null {
  if (typeof breadcrumb.data?.url === 'string') {
    breadcrumb.data.url = stripUrlSecrets(breadcrumb.data.url)
  }
  if (typeof breadcrumb.data?.from === 'string') {
    breadcrumb.data.from = stripUrlSecrets(breadcrumb.data.from)
  }
  if (typeof breadcrumb.data?.to === 'string') {
    breadcrumb.data.to = stripUrlSecrets(breadcrumb.data.to)
  }
  // console のパンくずは引数をそのまま持つ。何が載るか読めないので丸ごと落とす。
  if (breadcrumb.category === 'console') return null
  return breadcrumb
}

export function sentrySharedOptions() {
  const dsn =
    process.env.SENTRY_DSN?.trim() ||
    process.env.NEXT_PUBLIC_SENTRY_DSN?.trim() ||
    ''
  if (!dsn) return null

  return {
    dsn,
    // NEXT_PUBLIC_ が無いとクライアント側では undefined になり、
    // staging と production のイベントが Sentry 上で区別できない。
    environment:
      process.env.APP_ENV ||
      process.env.NEXT_PUBLIC_APP_ENV ||
      process.env.NODE_ENV ||
      'development',
    release: process.env.SENTRY_RELEASE || process.env.NEXT_PUBLIC_SENTRY_RELEASE || undefined,
    sendDefaultPii: false,
    tracesSampleRate: 0,
    beforeSend: scrubSentryEvent,
    beforeBreadcrumb: scrubSentryBreadcrumb,
  }
}
