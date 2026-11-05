import PageContent from './page-content'
import { requireAdminUser } from '@/lib/auth/server'

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  await requireAdminUser()
  const { id } = await params
  return <PageContent studentId={id} />
}
