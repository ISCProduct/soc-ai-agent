import PageContent from './page-content'
import { LandingContent } from '@/components/landing/LandingContent'
import { getSessionUser } from '@/lib/auth/server'

// 未ログインは公開LP、ログイン済みは従来の診断画面（#1653）。
//
// 以前は requireSessionUser() で未ログインを即 /login へ飛ばしていたため、
// 製品を説明する公開ページが1枚も存在しなかった。
//
// getSessionUser() は Cookie が無ければ Backend を呼ばずに null を返すので
// （lib/auth/server.ts の getSessionCredentials）、Backend 停止中でもLPは出る。
// 本番は展示会運用で desired=0 から起動するため、この性質に依存している。
export default async function Page() {
  const user = await getSessionUser()
  if (!user) return <LandingContent />
  return <PageContent />
}
