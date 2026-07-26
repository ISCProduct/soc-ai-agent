'use client'

import { useCompanyResults } from './company-results/hooks/useCompanyResults'
import { CompanyResultsLoading } from './company-results/components/CompanyResultsLoading'
import { CompanyResultsContent } from './company-results/components/CompanyResultsContent'
import type { CompanyResultsProps } from './company-results/types'

/**
 * 適性診断結果に基づく企業マッチング一覧。
 * 状態・副作用は useCompanyResults、表示は各コンポーネントに委譲する。
 */
export function CompanyResults({ userData, onResetAction }: CompanyResultsProps) {
  const { companies, loading, isProvisional, handleShowDetail } = useCompanyResults(userData)

  return (
    <div className="max-w-5xl mx-auto">
      {loading ? (
        <CompanyResultsLoading />
      ) : (
        <CompanyResultsContent
          userData={userData}
          companies={companies}
          isProvisional={isProvisional}
          onShowDetail={handleShowDetail}
          onResetAction={onResetAction}
        />
      )}
    </div>
  )
}
