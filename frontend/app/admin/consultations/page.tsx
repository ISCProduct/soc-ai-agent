import PageContent from './page-content'
import { requireAdminUser } from '@/lib/auth/server'

export default async function Page() {
  await requireAdminUser()
  return <PageContent />
}
