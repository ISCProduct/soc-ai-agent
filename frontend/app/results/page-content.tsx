'use client'

import { Suspense, type ReactNode } from 'react'
import dynamic from 'next/dynamic'
import { Box, CircularProgress } from '@mui/material'
import { PageLoading } from '@/components/common/PageLoading'
import { CONTENT_MAX_WIDTH } from '@/lib/design/tokens'
import { useResultsData } from './hooks/useResultsData'
import {
  ResultsLoadingView,
  ResultsErrorView,
  ResultsNoMatchView,
} from './components/ResultsStatusViews'
import ResultsListView from './components/ResultsListView'
import AnalysisScoreCard from './components/AnalysisScoreCard'
import { LowMatchConfirmDialog } from '@/components/LowMatchConfirmDialog'

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
 * エラー・0件は一覧と同じく、余白を全幅の外側に置き内側だけ CONTENT_MAX_WIDTH にする。
 * 同じボックスに maxWidth と padding を置くと、分析カードが広幅で 32px(sm) / 16px(xs) 右へずれる。
 */
function ResultsFallbackColumn({ children }: { children: ReactNode }) {
  return (
    <Box sx={{ width: '100%', p: { xs: 2, sm: 4 } }}>
      <Box sx={{ maxWidth: CONTENT_MAX_WIDTH, mx: 'auto', width: '100%' }}>
        {children}
      </Box>
    </Box>
  )
}

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
    diagnosisSummary,
    evaluatedCategories,
    minMatchedAxisCount,
    diagnosisConfidence,
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
    lowMatchTarget,
    cancelLowMatchApply,
    confirmLowMatchApply,
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
      <ResultsFallbackColumn>
        <AnalysisScoreCard
          analysisScores={analysisScores}
          scoreComment={scoreComment}
          analysisError={analysisError}
          onRetryAnalysis={handleRetryAnalysis}
        />
        <ResultsErrorView
          error={error}
          onBack={handleBack}
          onReset={handleReset}
        />
      </ResultsFallbackColumn>
    )
  }

  if (companies.length === 0) {
    return (
      <ResultsFallbackColumn>
        <AnalysisScoreCard
          analysisScores={analysisScores}
          scoreComment={scoreComment}
          analysisError={analysisError}
          onRetryAnalysis={handleRetryAnalysis}
        />
        <ResultsNoMatchView onReset={handleReset} />
      </ResultsFallbackColumn>
    )
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
    <>
    <ResultsListView
      companies={companies}
      isProvisional={isProvisional}
      diagnosisSummary={diagnosisSummary}
      evaluatedCategories={evaluatedCategories}
      minMatchedAxisCount={minMatchedAxisCount}
      diagnosisConfidence={diagnosisConfidence}
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
    {/* マッチ度が低い企業への応募前の確認(#1028)。応募はブロックしない */}
    {lowMatchTarget && (
      <LowMatchConfirmDialog
        open
        companyName={lowMatchTarget.name}
        matchScore={lowMatchTarget.matchScore}
        onCancel={cancelLowMatchApply}
        onConfirm={confirmLowMatchApply}
      />
    )}
    </>
  )
}

export default function PageContent() {
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
