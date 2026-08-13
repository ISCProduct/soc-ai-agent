'use client'

import { Suspense } from 'react'
import dynamic from 'next/dynamic'
import { Box, CircularProgress } from '@mui/material'
import { PageLoading } from '@/components/common/PageLoading'
import { useResultsData } from './hooks/useResultsData'
import {
  ResultsLoadingView,
  ResultsErrorView,
  ResultsNoMatchView,
} from './components/ResultsStatusViews'
import ResultsListView from './components/ResultsListView'

/** ReactFlow 依存の詳細ビューは一覧表示では不要のため遅延ロード */
const CompanyDetailView = dynamic(() => import('./components/CompanyDetailView'), {
  ssr: false,
  loading: () => (
    <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <CircularProgress />
    </Box>
  ),
})

/**
 * マッチング結果ページのオーケストレーション。
 * データ取得・操作は useResultsData、表示は各コンポーネントに委譲する。
 */
function ResultsContent() {
  const {
    mounted,
    userId,
    sessionId,
    companies,
    loading,
    error,
    isProvisional,
    jobSuitabilityComment,
    suggestedRoles,
    scoreComment,
    analysisScores,
    analysisError,
    handleRetryAnalysis,
    selectedCompany,
    setSelectedCompany,
    detailTab,
    setDetailTab,
    relations,
    marketInfo,
    diagramLoading,
    diagramError,
    emailSending,
    favoritingId,
    applyingId,
    snackbar,
    isGuestUser,
    handleSendEmail,
    handleBack,
    handleReset,
    handleToggleFavorite,
    handleApply,
    handleCloseDetail,
    handleCloseSnackbar,
    navigate,
  } = useResultsData()

  if (!mounted || !userId || !sessionId) {
    return <PageLoading message="マッチング結果を準備しています..." />
  }

  if (loading) {
    return <ResultsLoadingView />
  }

  if (error) {
    return (
      <ResultsErrorView
        error={error}
        onBack={handleBack}
        onReset={handleReset}
      />
    )
  }

  if (companies.length === 0) {
    return <ResultsNoMatchView onReset={handleReset} />
  }

  if (selectedCompany) {
    return (
      <CompanyDetailView
        company={selectedCompany}
        detailTab={detailTab}
        onDetailTabChange={setDetailTab}
        onBack={handleCloseDetail}
        relations={relations}
        marketInfo={marketInfo}
        diagramLoading={diagramLoading}
        diagramError={diagramError}
      />
    )
  }

  return (
    <ResultsListView
      companies={companies}
      isProvisional={isProvisional}
      analysisScores={analysisScores}
      scoreComment={scoreComment}
      analysisError={analysisError}
      onRetryAnalysis={handleRetryAnalysis}
      jobSuitabilityComment={jobSuitabilityComment}
      suggestedRoles={suggestedRoles}
      emailSending={emailSending}
      favoritingId={favoritingId}
      applyingId={applyingId}
      snackbar={snackbar}
      isGuestUser={isGuestUser}
      onCloseSnackbar={handleCloseSnackbar}
      onBack={handleBack}
      onReset={handleReset}
      onSendEmail={handleSendEmail}
      onSelectCompany={setSelectedCompany}
      onToggleFavorite={handleToggleFavorite}
      onApply={handleApply}
      onNavigate={navigate}
    />
  )
}

export default function ResultsPage() {
  return (
    <Suspense fallback={
      <Box sx={{
        height: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}>
        <CircularProgress size={60} />
      </Box>
    }>
      <ResultsContent />
    </Suspense>
  )
}
