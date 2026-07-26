'use client'

import { Suspense } from 'react'
import { Box } from '@mui/material'
import { PageLoading } from '@/components/common/PageLoading'
import { useResumePage } from './hooks/useResumePage'
import { ResumePageHeader } from './components/ResumePageHeader'
import { ResumeUploadForm } from './components/ResumeUploadForm'
import { ResumeReviewForm } from './components/ResumeReviewForm'
import { ResumeRagReport } from './components/ResumeRagReport'
import { ResumeReviewResults } from './components/ResumeReviewResults'

/**
 * 履歴書レビューページのオーケストレーション。
 * データ取得・操作は useResumePage、表示は各コンポーネントに委譲する。
 */
function ResumeContent() {
  const {
    router,
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
    review,
    ragReport,
    handleReview,
    scoresBefore,
    scoresAfter,
    annotateError,
    handleDownload,
  } = useResumePage()

  return (
    <Box sx={{ p: { xs: 2, sm: 4 }, maxWidth: 900, mx: 'auto' }}>
      <ResumePageHeader
        router={router}
        prefilledCompany={prefilledCompany}
        prefilledIndustry={prefilledIndustry}
      />

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
        reviewCompleted={!!review}
        onReview={handleReview}
      />

      <ResumeRagReport ragReport={ragReport} reviewLoading={reviewLoading} />

      <ResumeReviewResults
        review={review}
        scoresBefore={scoresBefore}
        scoresAfter={scoresAfter}
        annotateError={annotateError}
        onDownload={handleDownload}
      />
    </Box>
  )
}

export default function ResumePage() {
  return (
    <Suspense fallback={<PageLoading message="履歴書画面を準備しています..." />}>
      <ResumeContent />
    </Suspense>
  )
}
