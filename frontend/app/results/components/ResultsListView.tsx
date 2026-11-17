'use client'

import type { MouseEvent } from 'react'
import { Alert, Box, Button, Snackbar, useMediaQuery, useTheme } from '@mui/material'
import type { AnalysisScores, Company, SnackbarState, SuggestedRole } from '../types'
import { CONTENT_MAX_WIDTH, UI, scaleBand } from '@/lib/design/tokens'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import { ABOVE_BOTTOM_NAV_SX } from '@/lib/sidebar-nav'
import { AnalysisErrorAlert } from './list/AnalysisErrorAlert'
import { AnswerCoveragePanel } from './list/AnswerCoveragePanel'
import { CompanyList } from './list/CompanyList'
import { maxMatchedAxisCount } from './list/companyRules'
import { EmailButton } from './list/EmailButton'
import { NextStepBar } from './list/NextStepBar'
import { ResultsFooterLinks } from './list/ResultsFooterLinks'
import { ResultsHeader } from './list/ResultsHeader'
import { RolesPanel } from './list/RolesPanel'
import type { CompanyRowContext } from './list/rowContext'

export interface ResultsListViewProps {
  companies: Company[]
  isProvisional: boolean
  diagnosisSummary?: string | null
  evaluatedCategories?: number | null
  minMatchedAxisCount?: number | null
  diagnosisConfidence?: number | null
  analysisScores: AnalysisScores | null
  scoreComment: string
  analysisError: string | null
  onRetryAnalysis: () => void
  jobSuitabilityComment: string
  suggestedRoles: SuggestedRole[]
  emailSending: boolean
  favoritingId: number | null
  applyingId: number | null
  snackbar: SnackbarState
  isGuestUser: boolean
  /** 応募を受け付けているか。閉じている間は応募ボタンを「準備中」で無効にする */
  applyEnabled: boolean
  onCloseSnackbar: () => void
  onBack: () => void
  onReset: () => void
  onSendEmail: () => void
  onSelectCompany: (company: Company) => void
  onToggleFavorite: (e: MouseEvent, company: Company) => void
  onApply: (e: MouseEvent, company: Company) => void
  onNavigate: (path: string) => void
}

/**
 * マッチング結果一覧。部品を並べるだけにし、判定は list/ 配下の純関数に置く。
 * 並びは Figma「診断結果 改善案」v4。PC は 企業一覧（左）と「わかっていること」「向いていそうな職種」（右）の2カラム、
 * SP は 次にやること → わかっていること → 企業一覧 → 向いていそうな職種 → メール・リンク の縦積み。
 */
export default function ResultsListView({
  companies,
  isProvisional,
  diagnosisSummary = null,
  minMatchedAxisCount: _minMatchedAxisCount = null,
  evaluatedCategories: _evaluatedCategories = null,
  diagnosisConfidence: _diagnosisConfidence = null,
  analysisScores,
  scoreComment: _scoreComment = '',
  analysisError,
  onRetryAnalysis,
  jobSuitabilityComment,
  suggestedRoles,
  emailSending,
  favoritingId,
  applyingId,
  snackbar,
  isGuestUser,
  applyEnabled,
  onCloseSnackbar,
  onBack,
  onReset,
  onSendEmail,
  onSelectCompany,
  onToggleFavorite,
  onApply,
  onNavigate,
}: ResultsListViewProps) {
  const isDesktop = useMediaQuery(useTheme().breakpoints.up('md'), { noSsr: true })

  const band = scaleBand(companies.map((c) => c.matchScore))
  const ctx: CompanyRowContext = {
    isDesktop, maxAxisCount: maxMatchedAxisCount(companies), isProvisional, isGuestUser, applyEnabled,
    favoritingId, applyingId, onSelectCompany, onToggleFavorite, onApply, onNavigate,
  }
  const email = <EmailButton isGuestUser={isGuestUser} emailSending={emailSending} onSendEmail={onSendEmail} fullWidth={!isDesktop} />
  const coverage = analysisScores ? <AnswerCoveragePanel scores={analysisScores} layout={isDesktop ? 'list' : 'row'} /> : null
  const roles = <RolesPanel jobSuitabilityComment={jobSuitabilityComment} suggestedRoles={suggestedRoles} />

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', overflow: 'hidden', backgroundColor: UI.paper }}>
      <ResultsHeader
        companyCount={companies.length}
        isProvisional={isProvisional}
        diagnosisSummary={diagnosisSummary}
        band={band}
        isGuestUser={isGuestUser}
        emailAction={isDesktop ? email : null}
        onBack={onBack}
        onNavigate={onNavigate}
      />
      <NextStepBar analysisScores={analysisScores} isProvisional={isProvisional} onBack={onBack} />
      <Box sx={{ flexGrow: 1, overflowY: 'auto', px: { xs: 2, md: 4 }, pt: { xs: 2, md: 4 }, pb: { xs: 2, md: 7 } }}>
        <Box sx={{ maxWidth: CONTENT_MAX_WIDTH, mx: 'auto', display: 'flex', alignItems: 'flex-start', gap: 4 }}>
          <Box sx={{ flex: 1, minWidth: 0, display: 'grid', gap: { xs: 2, md: 2.5 } }}>
            <AnalysisErrorAlert analysisError={analysisError} onRetryAnalysis={onRetryAnalysis} />
            {!isDesktop && coverage}
            <CompanyList companies={companies} ctx={ctx} />
            {!isDesktop && roles}
            <ResultsFooterLinks isGuestUser={isGuestUser} emailAction={isDesktop ? null : email} onReset={onReset} onNavigate={onNavigate} />
          </Box>
          {isDesktop && (
            <Box sx={{ width: 340, flexShrink: 0, display: 'grid', gap: 2 }}>
              {coverage}
              {roles}
            </Box>
          )}
        </Box>
      </Box>

      {/* 手書きの pb ではなく共通部品で余白を取る（#1570） */}
      <BottomNavSpacer />
      <Snackbar
        open={snackbar.open}
        autoHideDuration={5000}
        onClose={onCloseSnackbar}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
        sx={ABOVE_BOTTOM_NAV_SX}
      >
        <Alert
          severity={snackbar.severity}
          onClose={onCloseSnackbar}
          action={
            snackbar.actionHref ? (
              <Button color="inherit" size="small" onClick={() => onNavigate(snackbar.actionHref!)}>
                {snackbar.actionLabel}
              </Button>
            ) : undefined
          }
        >
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  )
}
