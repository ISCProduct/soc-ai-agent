import PageContent from './page-content'
import { requireAdminUser } from '@/lib/server-auth'

export default async function Page() {
  await requireAdminUser()
  return <PageContent />
}
