'use client'

import type { MouseEvent } from 'react'
import {
  Box,
  Typography,
  Button,
  Card,
  CardContent,
  Stack,
  Avatar,
  Chip,
  Snackbar,
  Alert,
  IconButton,
  Tooltip,
} from '@mui/material'
import {
  ArrowBack,
  LocationOn,
  People,
  TrendingUp as TrendingUpIcon,
  Refresh,
  Email,
  Favorite,
  FavoriteBorder,
} from '@mui/icons-material'
import type { AnalysisScores, Company, SnackbarState, SuggestedRole } from '../types'
import { buildInterviewQuery, buildResumeQuery, getTopCategoryScores } from '../utils'
import {
  GUEST_REGISTER_CTA_LABEL,
  GUEST_REGISTER_PATH,
  getGuestApplicationsButtonProps,
  getGuestEmailButtonProps,
} from '@/lib/guest-limits'

export interface ResultsListViewProps {
  companies: Company[]
  isProvisional: boolean
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
 * マッチング結果一覧（ヘッダー・スコア・職種適性・企業カード・フッター・Snackbar）。
 */
export default function ResultsListView({
  companies,
  isProvisional,
  analysisScores,
  scoreComment,
  analysisError,
  onRetryAnalysis,
  jobSuitabilityComment,
  suggestedRoles,
  emailSending,
  favoritingId,
  applyingId,
  snackbar,
  isGuestUser,
  onCloseSnackbar,
  onBack,
  onReset,
  onSendEmail,
  onSelectCompany,
  onToggleFavorite,
  onApply,
  onNavigate,
}: ResultsListViewProps) {
  const guestEmailProps = getGuestEmailButtonProps(isGuestUser)
  const guestApplicationsProps = getGuestApplicationsButtonProps(isGuestUser)
  return (
    <Box sx={{
      height: '100vh',
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
      backgroundColor: '#fff',
    }}>
      {/* ヘッダー部分 */}
      <Box sx={{
        p: { xs: 2, sm: 3 },
        borderBottom: '1px solid #e0e0e0',
        backgroundColor: '#fff',
        flexShrink: 0,
      }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 1, mb: 2 }}>
          <Button variant="outlined" startIcon={<ArrowBack />} onClick={onBack}>
            チャットに戻る
          </Button>
          <Tooltip title={guestEmailProps.title} disableHoverListener={!guestEmailProps.disabled}>
            <span>
              <Button
                variant="contained"
                startIcon={<Email />}
                onClick={onSendEmail}
                disabled={emailSending || guestEmailProps.disabled}
                sx={{ width: { xs: '100%', sm: 'auto' } }}
              >
                {emailSending ? '送信中...' : '結果をメールで受け取る'}
              </Button>
            </span>
          </Tooltip>
        </Box>
        {isGuestUser && (
          <Alert
            severity="info"
            sx={{ mb: 2 }}
            action={
              <Button color="inherit" size="small" onClick={() => onNavigate(GUEST_REGISTER_PATH)}>
                {GUEST_REGISTER_CTA_LABEL}
              </Button>
            }
          >
            ゲスト利用中です。メール送信や選考管理はアカウント登録後に利用できます。
          </Alert>
        )}
        <Box sx={{ textAlign: 'center' }}>
          <Typography variant="h4" fontWeight="bold" gutterBottom sx={{ fontSize: { xs: '1.2rem', sm: '2.125rem' } }}>
            🎉 AI分析完了！適合企業を{companies.length}社に絞り込みました
          </Typography>
          {isProvisional && (
            <Chip label="暫定評価" color="warning" variant="outlined" sx={{ mb: 1 }} />
          )}
          <Typography variant="body1" color="text.secondary">
            AIによる詳細分析に基づいて、最適なIT企業をマッチングしました
          </Typography>
        </Box>
      </Box>

      {/* スクロール可能なコンテンツエリア */}
      <Box sx={{
        flexGrow: 1,
        overflowY: 'auto',
        p: { xs: 2, sm: 3 },
        backgroundColor: '#fafafa',
      }}>
        <Box sx={{ maxWidth: 1200, mx: 'auto' }}>
          {/* 分析データの部分取得失敗（サイレント失敗させず明示する） */}
          {analysisError && (
            <Alert
              severity="warning"
              sx={{ mb: 3 }}
              action={
                <Button color="inherit" size="small" startIcon={<Refresh />} onClick={onRetryAnalysis}>
                  再読み込み
                </Button>
              }
            >
              {analysisError}（4分析スコア・向いている職種は表示されません）
            </Alert>
          )}

          {/* 4分析スコアと総合コメント */}
          {(scoreComment || analysisScores) && (
            <Card elevation={2} sx={{ mb: 3, border: '2px solid', borderColor: 'primary.light', backgroundColor: '#f0f4ff' }}>
              <CardContent>
                <Typography variant="h6" fontWeight="bold" gutterBottom>
                  📊 4分析スコア
                </Typography>
                {analysisScores && (
                  <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)' }, gap: 2, mb: 2 }}>
                    {[
                      { label: '職種分析', value: analysisScores.job },
                      { label: '興味分析', value: analysisScores.interest },
                      { label: '適性分析', value: analysisScores.aptitude },
                      { label: '将来分析', value: analysisScores.future },
                    ].map(({ label, value }) => (
                      <Box key={label} sx={{ textAlign: 'center', bgcolor: '#fff', borderRadius: 2, p: 1.5, boxShadow: 1 }}>
                        <Typography variant="caption" color="text.secondary">{label}</Typography>
                        <Typography variant="h5" fontWeight="bold" color="primary.main">{value}%</Typography>
                      </Box>
                    ))}
                  </Box>
                )}
                {scoreComment && (
                  <Typography variant="body2" color="text.secondary">
                    {scoreComment}
                  </Typography>
                )}
              </CardContent>
            </Card>
          )}

          {/* 職種適性コメントセクション */}
          {(jobSuitabilityComment || suggestedRoles.length > 0) && (
            <Card elevation={2} sx={{ mb: 3, border: '2px solid', borderColor: 'success.light', backgroundColor: '#f0faf0' }}>
              <CardContent>
                <Typography variant="h6" fontWeight="bold" gutterBottom>
                  🎯 あなたに向いている職種
                </Typography>
                {jobSuitabilityComment && (
                  <Typography variant="body1" sx={{ mb: 2 }}>
                    {jobSuitabilityComment}
                  </Typography>
                )}
                {suggestedRoles.length > 0 && (
                  <Stack spacing={1.5}>
                    {suggestedRoles.map((role, i) => (
                      <Box key={i} sx={{ display: 'flex', gap: 1.5, alignItems: 'flex-start' }}>
                        <Chip label={role.title} color="success" variant="filled" sx={{ fontWeight: 'bold', flexShrink: 0 }} />
                        <Typography variant="body2" color="text.secondary" sx={{ pt: 0.5 }}>
                          → {role.reason}
                        </Typography>
                      </Box>
                    ))}
                  </Stack>
                )}
              </CardContent>
            </Card>
          )}

          {/* おすすめの次のステップ サマリー */}
          {companies.length > 0 && (
            <Card elevation={1} sx={{ mb: 2, border: '1px solid', borderColor: 'primary.light', bgcolor: '#f8f4ff' }}>
              <CardContent sx={{ py: 2 }}>
                <Typography variant="subtitle2" fontWeight="bold" gutterBottom>
                  おすすめの次のステップ
                </Typography>
                <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                  <Chip
                    label={`面接練習: ${companies[0].name}から始める`}
                    color="primary"
                    size="small"
                    onClick={() => onNavigate(`/interview?${buildInterviewQuery(companies[0])}`)}
                  />
                  <Chip label="企業詳細を確認する" variant="outlined" size="small" onClick={() => onSelectCompany(companies[0])} />
                  {companies.some((c) => !c.isFavorited) && (
                    <Chip
                      label="気になる企業をお気に入り登録"
                      variant="outlined"
                      size="small"
                      color="error"
                      onClick={(e) => {
                        const target = companies.find((c) => !c.isFavorited && c.matchId)
                        if (target) onToggleFavorite(e, target)
                      }}
                    />
                  )}
                </Stack>
              </CardContent>
            </Card>
          )}

          <Stack spacing={3}>
            {companies.map((company, index) => (
              <Card
                key={`${company.id}-${index}`}
                elevation={3}
                sx={{
                  border: '2px solid',
                  borderColor: 'primary.light',
                  cursor: 'pointer',
                  transition: 'all 0.3s ease',
                  '&:hover': {
                    borderColor: 'primary.main',
                    transform: 'translateY(-4px)',
                    boxShadow: 6,
                  },
                }}
                onClick={() => onSelectCompany(company)}
              >
                <CardContent>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                      <Avatar sx={{ bgcolor: 'primary.main', width: 40, height: 40, fontWeight: 'bold' }}>
                        {index + 1}
                      </Avatar>
                      <Box>
                        <Typography variant="h6" fontWeight="bold">
                          {company.name}
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                          {company.industry}
                        </Typography>
                      </Box>
                    </Box>
                    <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 0.5 }}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                        <Tooltip title={company.isFavorited ? 'お気に入り解除' : 'お気に入り登録'}>
                          <IconButton
                            size="small"
                            onClick={(e) => onToggleFavorite(e, company)}
                            disabled={favoritingId === company.matchId}
                            sx={{ color: company.isFavorited ? 'error.main' : 'action.disabled' }}
                          >
                            {company.isFavorited ? <Favorite fontSize="small" /> : <FavoriteBorder fontSize="small" />}
                          </IconButton>
                        </Tooltip>
                        <Typography variant="h4" color="primary.main" fontWeight="bold">
                          {company.matchScore}
                        </Typography>
                      </Box>
                      <Typography variant="caption" color="text.secondary">
                        適合度
                      </Typography>
                    </Box>
                  </Box>

                  <Typography variant="body2" sx={{ mb: 2 }}>
                    {company.description}
                  </Typography>

                  <Stack direction="row" spacing={2} sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <LocationOn fontSize="small" color="action" />
                      <Typography variant="body2" color="text.secondary">
                        {company.location}
                      </Typography>
                    </Box>
                    {company.employees && (
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <People fontSize="small" color="action" />
                        <Typography variant="body2" color="text.secondary">
                          {company.employees}
                        </Typography>
                      </Box>
                    )}
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <TrendingUpIcon fontSize="small" color="action" />
                      <Typography variant="body2" color="text.secondary">
                        {company.industry}
                      </Typography>
                    </Box>
                  </Stack>

                  {company.techStack && company.techStack.length > 0 && (
                    <Box sx={{ mb: 2 }}>
                      <Typography variant="caption" color="text.secondary" display="block" gutterBottom>
                        技術スタック:
                      </Typography>
                      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                        {company.techStack.map((tech, i) => (
                          <Chip key={i} label={tech} size="small" color="primary" variant="outlined" />
                        ))}
                      </Stack>
                    </Box>
                  )}

                  {company.tags && company.tags.length > 0 && (
                    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                      {company.tags.map((tag, i) => (
                        <Chip key={i} label={tag} size="small" />
                      ))}
                    </Stack>
                  )}

                  {company.categoryScores && (
                    <Box sx={{ mt: 2 }}>
                      <Typography variant="caption" color="text.secondary" display="block" gutterBottom>
                        カテゴリ別スコア（上位3項目）:
                      </Typography>
                      <Stack spacing={0.5}>
                        {getTopCategoryScores(company.categoryScores, 3).map(({ label, score }) => (
                          <Box key={label} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <Typography variant="caption" sx={{ minWidth: 100, color: 'text.secondary' }}>{label}</Typography>
                            <Box sx={{ flex: 1, bgcolor: 'grey.200', borderRadius: 1, height: 6, overflow: 'hidden' }}>
                              <Box sx={{ width: `${Math.round(score)}%`, bgcolor: 'primary.main', height: '100%', borderRadius: 1 }} />
                            </Box>
                            <Typography variant="caption" sx={{ minWidth: 30, textAlign: 'right', fontWeight: 'bold', color: 'primary.main' }}>
                              {Math.round(score)}
                            </Typography>
                          </Box>
                        ))}
                      </Stack>
                    </Box>
                  )}

                  <Box sx={{ mt: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Button
                      variant="contained"
                      size="small"
                      color="secondary"
                      onClick={(e) => {
                        e.stopPropagation()
                        onNavigate(`/interview?${buildInterviewQuery(company)}`)
                      }}
                    >
                      この企業の面接を練習する
                    </Button>
                    <Button
                      variant="outlined"
                      size="small"
                      onClick={(e) => {
                        e.stopPropagation()
                        onNavigate(`/Correlation-diagram?company_id=${company.id}`)
                      }}
                    >
                      関連企業を見る
                    </Button>
                    <Button
                      variant="outlined"
                      size="small"
                      color="success"
                      onClick={(e) => {
                        e.stopPropagation()
                        onNavigate(`/resume?${buildResumeQuery(company)}`)
                      }}
                    >
                      ES・職務経歴書を添削
                    </Button>
                    <Tooltip title={guestApplicationsProps.title} disableHoverListener={!guestApplicationsProps.disabled}>
                      <span>
                        <Button
                          variant={company.isApplied ? 'contained' : 'outlined'}
                          size="small"
                          color="primary"
                          disabled={company.isApplied || applyingId === company.matchId || guestApplicationsProps.disabled}
                          onClick={(e) => onApply(e, company)}
                        >
                          {company.isApplied ? '応募済み' : applyingId === company.matchId ? '応募中...' : '応募する'}
                        </Button>
                      </span>
                    </Tooltip>
                    <Typography variant="caption" color="primary" sx={{ fontWeight: 'bold' }}>
                      クリックして詳細を見る →
                    </Typography>
                  </Box>
                </CardContent>
              </Card>
            ))}
          </Stack>

          <Box sx={{ textAlign: 'center', mt: 4, mb: 4 }}>
            <Stack direction="row" spacing={2} justifyContent="center" flexWrap="wrap" useFlexGap>
              <Tooltip title={guestEmailProps.title} disableHoverListener={!guestEmailProps.disabled}>
                <span>
                  <Button
                    variant="contained"
                    startIcon={<Email />}
                    onClick={onSendEmail}
                    disabled={emailSending || guestEmailProps.disabled}
                  >
                    {emailSending ? '送信中...' : '結果をメールで受け取る'}
                  </Button>
                </span>
              </Tooltip>
              {isGuestUser && (
                <Button variant="contained" color="secondary" onClick={() => onNavigate(GUEST_REGISTER_PATH)}>
                  {GUEST_REGISTER_CTA_LABEL}
                </Button>
              )}
              <Tooltip title={guestApplicationsProps.title} disableHoverListener={!guestApplicationsProps.disabled}>
                <span>
                  <Button
                    variant="outlined"
                    size="large"
                    disabled={guestApplicationsProps.disabled}
                    onClick={() => onNavigate('/applications')}
                  >
                    選考管理を見る
                  </Button>
                </span>
              </Tooltip>
              <Button variant="outlined" size="large" startIcon={<Refresh />} onClick={onReset}>
                最初からやり直す
              </Button>
            </Stack>
          </Box>
        </Box>
      </Box>

      <Snackbar
        open={snackbar.open}
        autoHideDuration={5000}
        onClose={onCloseSnackbar}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert severity={snackbar.severity} onClose={onCloseSnackbar}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  )
}
