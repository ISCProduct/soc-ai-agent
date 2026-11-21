'use client'

import { ServiceErrorView } from '@/components/common/ServiceErrorView'

// 描画の例外（エラー境界）。接続の失敗とは限らないので、接続エラーの文言は出さない（#2213）。
// 「再試行」は retry で、このページを読み込み直して描画をやり直す（Next.js 16.3 以降の推奨。
// reset は読み込み直さずに描画だけをやり直すので、一時的な取得の失敗が直らない）。
export default function AppError({ retry }: { error: unknown; retry: () => void }) {
  return <ServiceErrorView kind="unexpected" onRetry={retry} />
}
