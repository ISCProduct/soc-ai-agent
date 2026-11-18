'use client'

import { Suspense, useEffect } from 'react'
import { Box } from '@mui/material'
import { PageLoading } from '@/components/common/PageLoading'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import { UI } from '@/lib/design/tokens'
import { useResumePage } from './hooks/useResumePage'
import { ResumePageHeader } from './components/ResumePageHeader'
import { ResumeDocumentCard } from './components/ResumeDocumentCard'
import { ResumeTargetCard } from './components/ResumeTargetCard'
import { ResumeStartBar } from './components/ResumeStartBar'
import { ResumeRunningCard } from './components/ResumeRunningCard'
import { ResumeReviewNotice } from './components/ResumeReviewNotice'
import { ResumeResultView } from './components/ResumeResultView'

/**
 * 履歴書・ES レビューのオーケストレーション。
 * 入力 → レビュー中 → 結果 の3つの状態を切り替える（Figma: 履歴書レビュー 改善案）。
 * データ取得・操作は useResumePage、表示は各コンポーネントに委譲する。
 */
function ResumeContent() {
  const {
    prefilledCompany,
    prefilledIndustry,
    sourceUrl,
    file,
    loading,
    uploadError,
    documentId,
    selectFile,
    loadFromUrl,
    resetDocument,
    cancelReview,
    targetMode,
    chooseTargetMode,
    companyQuery,
    handleCompanyQueryChange,
    companyName,
    companyValidated,
    companySearchLoading,
    companySearchError,
    selectedCompanyMeta,
    companyCandidates,
    selectCompany,
    handleSearchCompanies,
    jobTitle,
    setJobTitle,
    candidateType,
    setCandidateType,
    reviewLoading,
    reviewError,
    reviewInterrupted,
    reviewProgress,
    review,
    ragReport,
    handleReview,
    scoresBefore,
    scoresAfter,
    annotateError,
    handleDownload,
  } = useResumePage()

  const phase = reviewLoading ? 'running' : review ? 'result' : 'input'
  const step = phase === 'input' ? (documentId === null ? 1 : 2) : 3

  const fileName = file?.name ?? (sourceUrl ? 'Google Docs の書類' : '')
  const targetLabel = targetMode === 'company' ? companyName : jobTitle.trim()
  const candidateLabel = candidateType === 'mid_career' ? '中途' : '新卒'

  const targetReady = targetMode === 'company' ? companyValidated && !!companyName.trim() : !!jobTitle.trim()
  const ready = documentId !== null && targetReady
  const hint =
    documentId === null
      ? '書類を選ぶと、レビューを始められます'
      : targetMode === 'company'
        ? '応募先の企業を選ぶと、レビューを始められます'
        : '応募職種を入力すると、レビューを始められます'

  // 状態が変わったら先頭へ戻す（結果が画面の途中から始まらないように）
  useEffect(() => {
    window.scrollTo(0, 0)
  }, [phase])

  return (
    <Box sx={{ minHeight: '100%', bgcolor: UI.paper }}>
      <Box sx={{ maxWidth: 1200, mx: 'auto', px: { xs: 2, md: 0 } }}>
        <ResumePageHeader
          phase={phase}
          step={step}
          prefilledCompany={prefilledCompany}
          prefilledIndustry={prefilledIndustry}
          fileName={fileName}
          targetLabel={targetLabel}
          candidateLabel={candidateLabel}
          onReset={resetDocument}
        />

        {phase === 'input' && (
          <>
            <ResumeReviewNotice error={reviewError} interrupted={reviewInterrupted} canRetry={documentId !== null} onRetry={handleReview} />
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: { xs: 'minmax(0, 1fr)', md: 'minmax(0, 540px) minmax(0, 1fr)' },
                gap: { xs: 2, md: 3 },
                alignItems: 'start',
                pb: { xs: 2, md: 5 },
              }}
            >
              <ResumeDocumentCard
                fileName={fileName}
                documentId={documentId}
                loading={loading}
                uploadError={uploadError}
                onSelectFile={selectFile}
                onLoadUrl={loadFromUrl}
                onReset={resetDocument}
              />
              <ResumeTargetCard
                mode={targetMode}
                onModeChange={chooseTargetMode}
                companyQuery={companyQuery}
                onCompanyQueryChange={handleCompanyQueryChange}
                companyName={companyName}
                companyValidated={companyValidated}
                companySearchLoading={companySearchLoading}
                companySearchError={companySearchError}
                selectedCompanyMeta={selectedCompanyMeta}
                companyCandidates={companyCandidates}
                onSearchCompanies={handleSearchCompanies}
                onSelectCompany={selectCompany}
                jobTitle={jobTitle}
                onJobTitleChange={setJobTitle}
                candidateType={candidateType}
                onCandidateTypeChange={setCandidateType}
              />
            </Box>
          </>
        )}

        {phase === 'running' && (
          <ResumeRunningCard
            progress={reviewProgress}
            fileName={fileName}
            targetLabel={targetLabel}
            candidateLabel={candidateLabel}
            onCancel={cancelReview}
          />
        )}

        {phase === 'result' && review && (
          <ResumeResultView
            key={review.review.id}
            review={review}
            scoresBefore={scoresBefore}
            scoresAfter={scoresAfter}
            annotateError={annotateError}
            onDownload={handleDownload}
            ragReport={ragReport}
          />
        )}
      </Box>

      {phase === 'input' && (
        <ResumeStartBar
          ready={ready}
          summary={[fileName, targetLabel, candidateLabel].filter(Boolean).join('　・　')}
          hint={hint}
          onStart={handleReview}
        />
      )}
      {/* 固定の下部ナビに末尾が隠れないようにする（#1570） */}
      <BottomNavSpacer />
    </Box>
  )
}

export default function PageContent() {
  return (
    <Suspense fallback={<PageLoading message="履歴書画面を準備しています..." />}>
      <ResumeContent />
    </Suspense>
  )
}
