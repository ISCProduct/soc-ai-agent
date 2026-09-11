import { Suspense } from 'react'
import { CompanyPortalLoginContent } from '@/components/company-portal/CompanyPortalLoginContent'
import { PageLoading } from '@/components/common/PageLoading'

export default function CompanyPortalSignInPage() {
  return (
    <Suspense fallback={<PageLoading message="ログイン画面を準備しています..." />}>
      <CompanyPortalLoginContent />
    </Suspense>
  )
}
