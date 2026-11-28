import PageContent from './page-content'
import { requireSessionUser } from '@/lib/auth/server'

export default async function Page() {
  const user = await requireSessionUser()
  return <PageContent user={user} />
}
