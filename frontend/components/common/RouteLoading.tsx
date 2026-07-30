'use client'

import { PageLoading } from '@/components/common/PageLoading'

/** App Router の loading.tsx 用。遷移先チャンク取得中に即時表示する。 */
export default function RouteLoading({ message = '読み込み中...' }: { message?: string }) {
  return <PageLoading message={message} />
}
