import PageContent from './page-content'
import { requireSessionUser } from '@/lib/auth/server'

export default async function Page() {
  await requireSessionUser()
  return <PageContent />
}
