'use client'

import { useState, type MouseEvent } from 'react'
import { Alert, Box, Button, Snackbar, Stack, useMediaQuery, useTheme } from '@mui/material'
import type { AnalysisScores, Company, MatchFeedbackVerdict, SnackbarState, SuggestedRole } from '../types'
import { CONTENT_MAX_WIDTH, UI, scaleBand } from '@/lib/design/tokens'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import { ABOVE_BOTTOM_NAV_SX, SNACKBAR_AUTO_HIDE_MS } from '@/lib/sidebar-nav'
import { AnalysisErrorAlert } from './list/AnalysisErrorAlert'
import { AnswerCoveragePanel } from './list/AnswerCoveragePanel'
import { CompanyList } from './list/CompanyList'
import { maxMatchedAxisCount } from './list/companyRules'
import { EmailButton } from './list/EmailButton'
import { FavoritesSection } from './list/FavoritesSection'
import { NextStepBar } from './list/NextStepBar'
import { ResultsFooterLinks } from './list/ResultsFooterLinks'
import { ResultsHeader } from './list/ResultsHeader'
import { RolesPanel } from './list/RolesPanel'
import type { CompanyRowContext } from './list/rowContext'

export interface ResultsListViewProps {
  companies: Company[]
  /** 診断をまたいで1社にまとめたお気に入り。並びは適合度の高い順 */
  favoriteCompanies: Company[]
  favoritesLoading: boolean
  favoritesError: string | null
  onRetryFavorites: () => void
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
  /** 「合う／合わない」を送信中の推薦。送信中は押せない */
  feedbackingId?: number | null
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
  onUnfavoriteCompany: (e: MouseEvent, company: Company) => void
  /** 推薦への「合う／合わない」。省略すると、ボタンを出さない */
  onFeedback?: (e: MouseEvent, company: Company, verdict: MatchFeedbackVerdict) => void
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
  favoriteCompanies,
  favoritesLoading,
  favoritesError,
  onRetryFavorites,
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
  feedbackingId = null,
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
  onUnfavoriteCompany,
  onFeedback,
  onApply,
  onNavigate,
}: ResultsListViewProps) {
  const isDesktop = useMediaQuery(useTheme().breakpoints.up('md'), { noSsr: true })

  // お気に入りは過去の診断も含め、同じ企業は1行。中身は呼び出し側がまとめて渡す（#2301）
  const [favoritesOnly, setFavoritesOnly] = useState(false)
  const listedCompanies = favoritesOnly ? favoriteCompanies : companies
  const band = scaleBand(listedCompanies.map((c) => c.matchScore))
  const ctx: CompanyRowContext = {
    isDesktop, maxAxisCount: maxMatchedAxisCount(companies), isProvisional, isGuestUser, applyEnabled,
    favoritingId, applyingId, feedbackingId, onSelectCompany, onToggleFavorite, onFeedback, onApply, onNavigate,
  }
  // PC はヘッダー右上、SP は一覧の下に1つだけ出す
  const email = <EmailButton isGuestUser={isGuestUser} emailSending={emailSending} onSendEmail={onSendEmail} fullWidth={!isDesktop} />
  const coverage = analysisScores ? <AnswerCoveragePanel scores={analysisScores} layout={isDesktop ? 'list' : 'row'} /> : null
  const roles = <RolesPanel jobSuitabilityComment={jobSuitabilityComment} suggestedRoles={suggestedRoles} />

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column', overflow: 'hidden', backgroundColor: UI.paper }}>
      <ResultsHeader
        companyCount={listedCompanies.length}
        favoritesOnly={favoritesOnly}
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
            <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap" role="group" aria-label="表示する企業">
              <Button variant={favoritesOnly ? 'text' : 'contained'} aria-pressed={!favoritesOnly} onClick={() => setFavoritesOnly(false)} sx={{ minHeight: 44 }}>
                すべて
              </Button>
              <Button variant={favoritesOnly ? 'contained' : 'text'} aria-pressed={favoritesOnly} onClick={() => setFavoritesOnly(true)} sx={{ minHeight: 44 }}>
                お気に入りだけ（{favoriteCompanies.length}）
              </Button>
            </Stack>
            {favoritesOnly ? (
              <FavoritesSection
                companies={favoriteCompanies}
                loading={favoritesLoading}
                error={favoritesError}
                favoritingId={favoritingId}
                onRetry={onRetryFavorites}
                onShowAll={() => setFavoritesOnly(false)}
                onSelectCompany={onSelectCompany}
                onUnfavorite={onUnfavoriteCompany}
              />
            ) : (
              <CompanyList companies={companies} ctx={ctx} />
            )}
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
        autoHideDuration={SNACKBAR_AUTO_HIDE_MS}
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
