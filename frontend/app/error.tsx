'use client'

import { useEffect } from 'react'
import { usePathname } from 'next/navigation'
import * as Sentry from '@sentry/nextjs'
import { ServiceErrorView } from '@/components/common/ServiceErrorView'

// 描画の例外（エラー境界）。接続の失敗とは限らないので、接続エラーの文言は出さない（#2213）。
// 「再試行」は retry で、このページを読み込み直して描画をやり直す（Next.js 16.3 以降の推奨。
// reset は読み込み直さずに描画だけをやり直すので、一時的な取得の失敗が直らない）。
export default function AppError({ error, retry }: { error: unknown; retry: () => void }) {
  const pathname = usePathname() ?? ''

  // 原因を追えるよう、例外をログと監視に送る（画面には出さない。#2213 のレビュー）。
  // Sentry は DSN が無ければ何もしない。送る前に lib/sentry の scrub で URL のクエリなどを落とす。
  useEffect(() => {
    console.error(error)
    Sentry.captureException(error)
  }, [error])

  // このエラー境界は管理画面・企業ポータルでも出るので、学生以外には「先生に連絡」と案内しない
  const audience = pathname.startsWith('/admin') || pathname.startsWith('/company-portal') ? 'staff' : 'student'
  return <ServiceErrorView kind="unexpected" audience={audience} onRetry={retry} />
}
