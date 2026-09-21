'use client'

// @sentry/nextjs のクライアント初期化（DSN 未設定時は lib/sentry 側で no-op）
import '../sentry.client.config'

/** layout からマウントしてブラウザ側 Sentry を有効化する（#1185） */
export function SentryClientInit() {
  return null
}
