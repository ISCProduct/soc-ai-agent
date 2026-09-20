import type { ErrorEvent, EventHint } from '@sentry/nextjs'

const SENSITIVE_HEADERS = new Set([
  'authorization',
  'cookie',
  'x-admin-token',
  'x-user-token',
  'x-company-user-token',
  'x-internal-token',
])

/**
 * Sentry 送信前に認証ヘッダー・Cookie・ボディを落とす（#619 / #1185）。
 * 履歴書・チャット本文などの個人情報混入を防ぐ。
 */
export function scrubSentryEvent(event: ErrorEvent, _hint?: EventHint): ErrorEvent | null {
  if (event.request) {
    delete event.request.data
    delete event.request.cookies
    if (event.request.headers) {
      for (const key of Object.keys(event.request.headers)) {
        if (SENSITIVE_HEADERS.has(key.toLowerCase())) {
          delete event.request.headers[key]
        }
      }
    }
    if (event.request.query_string) {
      event.request.query_string = ''
    }
  }
  return event
}

export function sentrySharedOptions() {
  const dsn =
    process.env.SENTRY_DSN?.trim() ||
    process.env.NEXT_PUBLIC_SENTRY_DSN?.trim() ||
    ''
  if (!dsn) return null

  return {
    dsn,
    environment: process.env.APP_ENV || process.env.NODE_ENV || 'development',
    release: process.env.SENTRY_RELEASE || undefined,
    sendDefaultPii: false,
    tracesSampleRate: 0,
    beforeSend: scrubSentryEvent,
  }
}
