'use client'

import {
  Box,
  Button,
  IconButton,
  LinearProgress,
  Paper,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { InterviewReport, InterviewSession } from '@/lib/interview'
import InterviewSummary from './InterviewSummary'
import ScoreUpdateBanner, { WeightScore } from '@/components/ScoreUpdateBanner'
import { PRIMARY, BG_DARK } from '../constants'
import type { ReportStatus } from '../hooks/useInterviewSession'
import {
  GUEST_EMAIL_DISABLED_REASON,
  GUEST_REGISTER_CTA_LABEL,
  getGuestEmailButtonProps,
} from '@/lib/guest-limits'

export interface ReportScreenProps {
  onBack: () => void
  errorMessage: string | null
  reportStatus: ReportStatus
  report: InterviewReport | null
  scoresBefore: WeightScore[] | null
  scoresAfter: WeightScore[] | null
  /** メール送信・レポート紐付け先のセッション（現状 UI では onSendEmail 側で保持） */
  session: InterviewSession | null
  userId?: number
  emailSending: boolean
  emailSent: boolean
  onSendEmail: () => void
  /** タイムアウト / エラー時の再ポーリング */
  onRetryReport?: () => void
  /** 面接終了API(finishSession)が失敗したか(#1015) */
  finishFailed?: boolean
  /** finishSession失敗時の再試行 */
  onRetryFinish?: () => void
  /** ゲストユーザーはメール送信不可 */
  isGuest: boolean
  /** ゲスト向け登録導線への遷移 */
  onRegisterClick: () => void
  videoUploadStatus: 'idle' | 'uploading' | 'done' | 'error'
  videoUploadProgress: number
  videoSizeWarning: string | null
}

/**
 * REPORT SCREEN (finished)
 * 面接終了後のレポート表示・メール送信・動画アップロード状況を表示する画面。
 * データ取得（レポートポーリング・動画アップロード）は親コンポーネント側で行い、
 * このコンポーネントは受け取った props をそのまま描画する presentational component。
 */
export default function ReportScreen({
  onBack,
  errorMessage,
  reportStatus,
  report,
  scoresBefore,
  scoresAfter,
  userId,
  emailSending,
  emailSent,
  onSendEmail,
  onRetryReport,
  finishFailed,
  onRetryFinish,
  isGuest,
  onRegisterClick,
  videoUploadStatus,
  videoUploadProgress,
  videoSizeWarning,
}: ReportScreenProps) {
  return (
    <Box sx={{ minHeight: '100vh', bgcolor: BG_DARK, color: '#e8eaed', overflowY: 'auto', p: { xs: 2, md: 4 } }}>
      <Box sx={{ maxWidth: 720, mx: 'auto' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 4 }}>
          <IconButton sx={{ color: '#bdc1c6' }} onClick={onBack}>
            <ArrowBackIcon />
          </IconButton>
          <Typography variant="h5" sx={{ fontWeight: 700, color: '#e8eaed' }}>面接レポート</Typography>
        </Box>

        {errorMessage && (
          <Paper sx={{ bgcolor: 'rgba(234,67,53,0.15)', border: '1px solid rgba(234,67,53,0.4)', p: 2, mb: 2, borderRadius: 2 }}>
            <Typography variant="body2" sx={{ color: '#f28b82', mb: finishFailed && onRetryFinish ? 1.5 : 0 }}>{errorMessage}</Typography>
            {finishFailed && onRetryFinish && (
              <Button
                variant="contained"
                onClick={onRetryFinish}
                sx={{ bgcolor: PRIMARY, '&:hover': { bgcolor: '#d14f10' } }}
              >
                再試行
              </Button>
            )}
          </Paper>
        )}

        {reportStatus === 'pending' && (
          <Paper sx={{ bgcolor: '#2d2e31', border: '1px solid rgba(255,255,255,0.08)', p: 3, borderRadius: 2 }}>
            <Typography sx={{ color: '#e8eaed', mb: 1.5 }}>レポートを生成中です...</Typography>
            <LinearProgress sx={{ bgcolor: '#3c4043', '& .MuiLinearProgress-bar': { bgcolor: '#4285f4' } }} />
          </Paper>
        )}

        {reportStatus === 'ready' && report && (
          <Stack spacing={2}>
            <ScoreUpdateBanner
              beforeScores={scoresBefore}
              afterScores={scoresAfter}
              title="面接結果がプロフィールスコアに反映されました"
            />
            <InterviewSummary report={report} userId={userId} theme="dark" />
            {(() => {
              const guestEmailProps = getGuestEmailButtonProps(isGuest)
              return (
                <Stack spacing={1}>
                  <Tooltip title={guestEmailProps.title} disableHoverListener={!guestEmailProps.disabled}>
                    <span>
                      <Button
                        variant="outlined"
                        fullWidth
                        disabled={emailSending || emailSent || guestEmailProps.disabled}
                        onClick={onSendEmail}
                        sx={{ color: emailSent ? '#34a853' : PRIMARY, borderColor: emailSent ? '#34a853' : PRIMARY, '&:hover': { borderColor: PRIMARY, bgcolor: 'rgba(236,91,19,0.08)' } }}
                      >
                        {emailSent ? '✓ メールを送信しました' : emailSending ? '送信中...' : 'レポートをメールで受け取る'}
                      </Button>
                    </span>
                  </Tooltip>
                  {isGuest && (
                    <Typography variant="caption" sx={{ color: '#9aa0a6', textAlign: 'center' }}>
                      {GUEST_EMAIL_DISABLED_REASON}{' '}
                      <Button
                        size="small"
                        onClick={onRegisterClick}
                        sx={{ color: PRIMARY, textTransform: 'none', p: 0, minWidth: 0, verticalAlign: 'baseline' }}
                      >
                        {GUEST_REGISTER_CTA_LABEL}
                      </Button>
                    </Typography>
                  )}
                </Stack>
              )
            })()}
          </Stack>
        )}

        {(reportStatus === 'error' || reportStatus === 'timeout') && (
          <Paper sx={{ bgcolor: '#2d2e31', border: '1px solid rgba(234,67,53,0.3)', p: 3, borderRadius: 2 }}>
            <Typography variant="body2" sx={{ color: '#f28b82', mb: onRetryReport ? 2 : 0 }}>
              {reportStatus === 'timeout'
                ? 'レポート生成がタイムアウトしました。時間をおいて再試行してください。'
                : 'レポート生成に失敗しました。'}
            </Typography>
            {onRetryReport && (
              <Button
                variant="contained"
                onClick={onRetryReport}
                sx={{ bgcolor: PRIMARY, '&:hover': { bgcolor: '#d14f10' } }}
              >
                再試行
              </Button>
            )}
          </Paper>
        )}

        {/* Video upload status */}
        {videoUploadStatus !== 'idle' && (
          <Paper sx={{ bgcolor: '#2d2e31', border: '1px solid rgba(255,255,255,0.1)', p: 2, borderRadius: 2 }}>
            {videoSizeWarning && (
              <Typography variant="caption" sx={{ color: '#fdd663', display: 'block', mb: 1 }}>
                ⚠ {videoSizeWarning}
              </Typography>
            )}
            <Typography variant="body2" sx={{ color: videoUploadStatus === 'done' ? '#34a853' : videoUploadStatus === 'error' ? '#f28b82' : '#9aa0a6', mb: videoUploadStatus === 'uploading' ? 1 : 0 }}>
              {videoUploadStatus === 'uploading' && `動画をアップロード中... ${videoUploadProgress}%`}
              {videoUploadStatus === 'done' && '✓ 動画のアップロードが完了しました'}
              {videoUploadStatus === 'error' && '動画のアップロードに失敗しました。ネットワーク接続を確認してください'}
            </Typography>
            {videoUploadStatus === 'uploading' && (
              <LinearProgress
                variant="determinate"
                value={videoUploadProgress}
                sx={{ height: 6, borderRadius: 3, bgcolor: 'rgba(255,255,255,0.1)', '& .MuiLinearProgress-bar': { bgcolor: PRIMARY } }}
              />
            )}
          </Paper>
        )}
      </Box>
    </Box>
  )
}
