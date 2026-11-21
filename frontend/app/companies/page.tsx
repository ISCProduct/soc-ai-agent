import PageContent from './page-content'
import { requireSessionUser } from '@/lib/auth/server'

// 企業を探す（SOCAIAGENT-612）
export default async function Page() {
  await requireSessionUser()
  return <PageContent />
}
