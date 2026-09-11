import { StudentDetailContent } from '@/components/company-portal/StudentDetailContent'

export default async function CompanyPortalStudentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = await params
  return <StudentDetailContent userId={Number(id)} />
}
