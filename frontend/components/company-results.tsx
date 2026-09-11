'use client'

import { useCompanyResults } from './company-results/hooks/useCompanyResults'
import { CompanyResultsLoading } from './company-results/components/CompanyResultsLoading'
import { CompanyResultsContent } from './company-results/components/CompanyResultsContent'
import { ErrorAlert } from './common/ErrorAlert'
import type { CompanyResultsProps } from './company-results/types'

/**
 * 適性診断結果に基づく企業マッチング一覧。
 * 状態・副作用は useCompanyResults、表示は各コンポーネントに委譲する。
 */
export function CompanyResults({ userData, onResetAction }: CompanyResultsProps) {
  const { companies, loading, isProvisional, error, applyingId, handleShowDetail, handleApply } =
    useCompanyResults(userData)

  return (
    <div className="max-w-5xl mx-auto">
      {loading ? (
        <CompanyResultsLoading />
      ) : companies.length === 0 ? (
        <ErrorAlert error={error || '企業データを取得できませんでした。時間をおいて再度お試しください。'} />
      ) : (
        <>
          <ErrorAlert error={error} />
          <CompanyResultsContent
            userData={userData}
            companies={companies}
            isProvisional={isProvisional}
            applyingId={applyingId}
            onShowDetail={handleShowDetail}
            onApply={handleApply}
            onResetAction={onResetAction}
          />
        </>
      )}
    </div>
  )
}
