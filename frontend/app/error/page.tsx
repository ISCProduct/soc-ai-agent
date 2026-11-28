import type { Metadata } from 'next'
import { ServiceErrorView } from '@/components/common/ServiceErrorView'
import { safeReturnPath } from '@/lib/user-facing-error'

const TITLE = '接続できません | 就活AI'
const DESCRIPTION = 'サーバーに接続できませんでした。しばらくしてから、もう一度お試しください。'

// クエリの code（ステータスコード）は画面にもタイトルにも出さない（#2213）。
export const metadata: Metadata = {
  title: TITLE,
  description: DESCRIPTION,
  robots: { index: false, follow: false },
  openGraph: {
    title: TITLE,
    description: DESCRIPTION,
    locale: 'ja_JP',
    type: 'website',
  },
}

export default async function ServiceErrorPage({
  searchParams,
}: {
  searchParams: Promise<{ from?: string | string[] }>
}) {
  const { from } = await searchParams
  // 「再試行」は、エラーになったページ（from）へ戻って読み直す。同じサイトの中のパスだけを通し、無ければトップへ。
  const retryHref = safeReturnPath(typeof from === 'string' ? from : undefined) ?? '/'
  return <ServiceErrorView kind="connection" retryHref={retryHref} />
}
