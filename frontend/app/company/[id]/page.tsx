import PageContent from './page-content'
import { requireSessionUser } from '@/lib/server-auth'

export default async function Page() {
  await requireSessionUser()
  return <PageContent />
}
