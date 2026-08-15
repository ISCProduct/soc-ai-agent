import type { Metadata } from 'next'
import { ServiceErrorView } from '@/components/service-error-view'

const TITLE = '接続できません | 就活AI'
const DESCRIPTION = 'サーバーに接続できませんでした。しばらくしてから再試行してください。'

export async function generateMetadata({
  searchParams,
}: {
  searchParams: Promise<{ code?: string }>
}): Promise<Metadata> {
  const { code } = await searchParams
  const title = code ? `接続できません (${code}) | 就活AI` : TITLE
  return {
    title,
    description: DESCRIPTION,
    robots: { index: false, follow: false },
    openGraph: {
      title,
      description: DESCRIPTION,
      locale: 'ja_JP',
      type: 'website',
    },
  }
}

export default async function ServiceErrorPage({
  searchParams,
}: {
  searchParams: Promise<{ code?: string }>
}) {
  const { code } = await searchParams
  return <ServiceErrorView code={code} />
}
