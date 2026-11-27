'use client'

// 管理画面の企業一覧。状態はフック、表示は components/ の部品に分け、ここでは組み立てだけを行う。
import { useState } from 'react'
import { formatNumber } from '@/lib/i18n/format'
import Link from 'next/link'
import { Alert, Box, Button, Pagination } from '@mui/material'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel } from '@/components/admin/AdminPanel'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { PAGE_SIZE } from './company-list-model'
import { useCompanyList } from './hooks/useCompanyList'
import { useCompanyApprovals } from './hooks/useCompanyApprovals'
import { useCompanyBatchActions } from './hooks/useCompanyBatchActions'
import { useCompanyRowActions } from './hooks/useCompanyRowActions'
import { useLeaveGuard } from './hooks/useLeaveGuard'
import { CompanyUsageGuide } from './components/CompanyUsageGuide'
import { MissingBatchPanel } from './components/MissingBatchPanel'
import { CompanyFilterBar } from './components/CompanyFilterBar'
import { BulkPublishBar } from './components/BulkPublishBar'
import { CompanyList } from './components/CompanyList'
import { CompanyActionsMenu } from './components/CompanyActionsMenu'
import { AdvancedSettings } from './components/AdvancedSettings'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'

export default function PageContent() {
  const list = useCompanyList()
  const {
    isPlatform,
    companies,
    total,
    page,
    error,
    setError,
    selectedIds,
    setSelectedIds,
    schoolId,
    reloadCurrentList,
    filterSummary,
  } = list
  const [fetchMessage, setFetchMessage] = useState('')
  const [fetchSeverity, setFetchSeverity] = useState<'success' | 'warning'>('success')
  const { approvedCompanyIds, approvalLoading, approvalBusyId, approvalError, toggleCompanyApproval } =
    useCompanyApprovals(schoolId)
  const batch = useCompanyBatchActions({
    coverage: list.coverage,
    setCoverage: list.setCoverage,
    fetchCoverage: list.fetchCoverage,
    reloadCurrentList,
    setError,
    setFetchMessage,
    setFetchSeverity,
  })
  const row = useCompanyRowActions({
    companies,
    selectedIds,
    setSelectedIds,
    reloadCurrentList,
    setError,
    setFetchMessage,
    setFetchSeverity,
  })
  const { warming, missingBatchLoading } = batch
  const { fetchPrimaryId, bulkPublishing } = row

  const pageCount = Math.ceil(total / PAGE_SIZE)
  const busy = warming || missingBatchLoading || fetchPrimaryId !== null || bulkPublishing
  const selectedCount = selectedIds.length

  useLeaveGuard(busy)

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="企業情報の管理"
        description={
          filterSummary
            ? `${filterSummary} の結果 ${formatNumber(total)} 件`
            : `学生課の先生が、学生に見せる企業情報を登録・確認・公開するための画面です。登録 ${formatNumber(total)} 社`
        }
        backHref={busy ? undefined : '/admin'}
        actions={
          isPlatform ? (
            <Button
              variant="contained"
              component={Link}
              href="/admin/companies/new"
              disableElevation
              disabled={busy}
            >
              企業を追加
            </Button>
          ) : undefined
        }
      />

      <ErrorAlert error={error} />
      {approvalError ? (
        <Alert severity="warning" sx={{ mb: 2 }}>
          {approvalError}
        </Alert>
      ) : null}
      {fetchMessage && (
        <Alert severity={fetchSeverity} sx={{ mb: 2 }} onClose={() => setFetchMessage('')}>
          {fetchMessage}
        </Alert>
      )}

      <CompanyUsageGuide isPlatform={isPlatform} />

      {isPlatform && (
        <MissingBatchPanel
          busy={busy}
          missingBatchLoading={missingBatchLoading}
          batchProgress={batch.batchProgress}
          handleFetchMissingBatch={batch.handleFetchMissingBatch}
        />
      )}

      <AdminPanel title="企業一覧">
        <CompanyFilterBar
          searchInput={list.searchInput}
          setSearchInput={list.setSearchInput}
          filterIndustry={list.filterIndustry}
          setFilterIndustry={list.setFilterIndustry}
          industries={list.industries}
          schoolId={schoolId}
          setSchoolId={list.setSchoolId}
          hasActiveFilters={list.hasActiveFilters}
          resetFilters={list.resetFilters}
          filterStatus={list.filterStatus}
          setFilterStatus={list.setFilterStatus}
          filterReadiness={list.filterReadiness}
          setFilterReadiness={list.setFilterReadiness}
          groupByIndustry={list.groupByIndustry}
          setGroupByIndustry={list.setGroupByIndustry}
          setPage={list.setPage}
        />
        <ConfirmDialog
          open={row.bulkPublishTargets !== null}
          title="企業を一括公開"
          message={`選択した ${row.bulkPublishTargets?.length ?? 0} 社を学生に公開しますか？`}
          confirmLabel="公開する"
          busy={row.bulkPublishing}
          onConfirm={row.confirmBulkPublish}
          onCancel={row.cancelBulkPublish}
        />
        <BulkPublishBar
          selectableOnPage={list.selectableOnPage}
          selectedCount={selectedCount}
          selectedSelectableCount={list.selectedSelectableCount}
          allSelectableSelected={list.allSelectableSelected}
          toggleSelectAllSelectable={list.toggleSelectAllSelectable}
          setSelectedIds={setSelectedIds}
          handleBulkPublish={row.handleBulkPublish}
          busy={busy}
          bulkPublishing={bulkPublishing}
        />

        <CompanyList
          companies={companies}
          companyGroups={list.companyGroups}
          hasActiveFilters={list.hasActiveFilters}
          resetFilters={list.resetFilters}
          rowProps={{
            isPlatform,
            groupByIndustry: list.groupByIndustry,
            busy,
            fetchPrimaryId,
            selectedIds,
            toggleSelect: list.toggleSelect,
            schoolId,
            approvedCompanyIds,
            approvalLoading,
            approvalBusyId,
            toggleCompanyApproval,
            setFilterIndustry: list.setFilterIndustry,
            setPage: list.setPage,
            handleFetchPrimary: row.handleFetchPrimary,
            handlePublish: row.handlePublish,
            setMenuAnchor: row.setMenuAnchor,
          }}
        />

        {pageCount > 1 && (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 2.5, borderTop: '1px solid', borderColor: 'divider' }}>
            <Pagination count={pageCount} page={page} onChange={list.handlePageChange} color="primary" />
          </Box>
        )}
      </AdminPanel>


      <CompanyActionsMenu
        menuAnchor={row.menuAnchor}
        setMenuAnchor={row.setMenuAnchor}
        isPlatform={isPlatform}
        busy={busy}
        handleFetchPrimary={row.handleFetchPrimary}
        handlePublish={row.handlePublish}
        handleReject={row.handleReject}
        handleSetVerified={row.handleSetVerified}
      />

      <AdvancedSettings
        isPlatform={isPlatform}
        busy={busy}
        coverage={list.coverage}
        warming={warming}
        warmMessage={batch.warmMessage}
        batchProgress={batch.batchProgress}
        handleSeedL1={batch.handleSeedL1}
        handleWarmL1={batch.handleWarmL1}
      />
    </PageContainer>
  )
}
