'use client'

import { Suspense } from 'react'
import { Box } from '@mui/material'
import { PageLoading } from '@/components/common/PageLoading'
import { EmptyState } from '@/components/common/EmptyState'
import { useResumePage } from './hooks/useResumePage'
import { ResumePageHeader } from './components/ResumePageHeader'
import { ResumeUploadForm } from './components/ResumeUploadForm'
import { ResumeReviewForm } from './components/ResumeReviewForm'
import { ResumeRagReport } from './components/ResumeRagReport'
import { ResumeSection, type ResumeSectionState } from './components/ResumeSection'
import { ResumeReviewResults } from './components/ResumeReviewResults'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'

/**
 * 履歴書レビューページのオーケストレーション。
 * データ取得・操作は useResumePage、表示は各コンポーネントに委譲する。
 */
function ResumeContent() {
  const {
    prefilledCompany,
    prefilledIndustry,
    sourceType,
    setSourceType,
    sourceUrl,
    setSourceUrl,
    file,
    setFile,
    loading,
    uploadError,
    documentId,
    handleUpload,
    companyQuery,
    handleCompanyQueryChange,
    companyName,
    companyValidated,
    companySearchLoading,
    companySearchError,
    selectedCompanyMeta,
    companyCandidates,
    selectCompany,
    clearSelectedCompany,
    handleSearchCompanies,
    jobTitle,
    setJobTitle,
    candidateType,
    setCandidateType,
    reviewLoading,
    reviewError,
    reviewInterrupted,
    review,
    ragReport,
    handleReview,
    scoresBefore,
    scoresAfter,
    annotateError,
    handleDownload,
  } = useResumePage()

  const conditionState: ResumeSectionState = !documentId ? 'todo' : review ? 'done' : 'active'
  const resultState: ResumeSectionState = review ? 'done' : reviewLoading || ragReport || reviewInterrupted ? 'active' : 'todo'

  return (
    <Box sx={{ p: { xs: 2, sm: 4 }, maxWidth: 900, mx: 'auto' }}>
      <ResumePageHeader
        prefilledCompany={prefilledCompany}
        prefilledIndustry={prefilledIndustry}
      />

      <ResumeSection step={1} title="書類を選ぶ" state={documentId !== null ? 'done' : 'active'}>
        <ResumeUploadForm
          sourceType={sourceType}
          onSourceTypeChange={setSourceType}
          sourceUrl={sourceUrl}
          onSourceUrlChange={setSourceUrl}
          file={file}
          onFileChange={setFile}
          loading={loading}
          uploadError={uploadError}
          documentId={documentId}
          onUpload={handleUpload}
        />

      </ResumeSection>

      <ResumeSection step={2} title="レビューの条件を決める" state={conditionState}>
        <ResumeReviewForm
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
          onClearSelectedCompany={clearSelectedCompany}
          jobTitle={jobTitle}
          onJobTitleChange={setJobTitle}
          candidateType={candidateType}
          onCandidateTypeChange={setCandidateType}
          documentId={documentId}
          reviewLoading={reviewLoading}
          ragReport={ragReport}
          reviewError={reviewError}
          reviewInterrupted={reviewInterrupted}
          reviewCompleted={!!review}
          onReview={handleReview}
        />

      </ResumeSection>

      <ResumeSection step={3} title="結果を見る" state={resultState}>
        {!ragReport && (
          <EmptyState
            dense
            title="まだレビュー結果がありません"
            description={
              reviewLoading
                ? 'レポートが書き始められると、ここに表示されます。'
                : '「レビューの条件」を決めて「レビューを生成」を押すと、ここに結果が出ます。'
            }
          />
        )}
        <ResumeRagReport ragReport={ragReport} reviewLoading={reviewLoading} interrupted={reviewInterrupted} />

        <ResumeReviewResults
          review={review}
          scoresBefore={scoresBefore}
          scoresAfter={scoresAfter}
          annotateError={annotateError}
          onDownload={handleDownload}
        />
      </ResumeSection>
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
