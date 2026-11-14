'use client'

import Link from 'next/link'
import {
  Box,
  Button,
  CircularProgress,
  IconButton,
  LinearProgress,
  Paper,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import HistoryIcon from '@mui/icons-material/History'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked'
import { InterviewReport, InterviewSession } from '@/lib/interview'
import InterviewSummary from './InterviewSummary'
import ScoreUpdateBanner, { WeightScore } from '@/components/feedback/ScoreUpdateBanner'
import { ErrorAlert } from '@/components/common/ErrorAlert'

import { UTTERANCE_SAVE_FAILED_MESSAGE } from '../utteranceSave'
import type { ReportStatus } from '../hooks/useInterviewSession'
import { REPORT_STAGE_STEPS, type ReportProgressView } from '../reportProgress'
import {
  GUEST_EMAIL_DISABLED_REASON,
  GUEST_REGISTER_CTA_LABEL,
  getGuestEmailButtonProps,
} from '@/lib/guest-limits'
import { UI } from '@/lib/design/tokens'

export interface ReportScreenProps {
  onBack: () => void
  errorMessage: string | null
  reportStatus: ReportStatus
  /** 生成の段階・%。バックエンドが返さない間は null（従来の不定表示） */
  reportProgress?: ReportProgressView | null
  /** サーバーが「失敗」と確定したか（タイムアウト・通信失敗とは文言を分ける） */
  reportFailedByServer?: boolean
  report: InterviewReport | null
  scoresBefore: WeightScore[] | null
  scoresAfter: WeightScore[] | null
  /** メール送信・レポート紐付け先のセッション（現状 UI では onSendEmail 側で保持） */
  session: InterviewSession | null
  userId?: number
  emailSending: boolean
  emailSent: boolean
  emailError: string
  onSendEmail: () => void
  /** タイムアウト / エラー時の再ポーリング */
  onRetryReport?: () => void
  /** 再生成API(regenerateReport)自体が失敗したときのメッセージ。握り潰さず即座に出す(#1476) */
  reportRetryError?: string
  /** 面接終了API(finishSession)が失敗したか(#1015) */
  finishFailed?: boolean
  /** finishSession失敗時の再試行 */
  onRetryFinish?: () => void
  /** 発話保存が再試行しても失敗したか。レポートが欠ける理由として必ず出す（#1476） */
  utteranceSaveFailed?: boolean
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
  reportProgress,
  reportFailedByServer,
  report,
  scoresBefore,
  scoresAfter,
  userId,
  emailSending,
  emailSent,
  emailError,
  onSendEmail,
  onRetryReport,
  reportRetryError,
  finishFailed,
  onRetryFinish,
  utteranceSaveFailed,
  isGuest,
  onRegisterClick,
  videoUploadStatus,
  videoUploadProgress,
  videoSizeWarning,
}: ReportScreenProps) {
  return (
    <Box sx={{ minHeight: '100vh', bgcolor: UI.nightBg, color: UI.nightInk, overflowY: 'auto', p: { xs: 2, md: 4 } }}>
      <Box sx={{ maxWidth: 720, mx: 'auto' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 4 }}>
          <IconButton sx={{ color: UI.nightSoft }} onClick={onBack} aria-label="前の画面へ戻る">
            <ArrowBackIcon />
          </IconButton>
          <Typography variant="h5" sx={{ fontWeight: 700, color: UI.nightInk, flex: 1 }}>面接レポート</Typography>
          <Button
            component={Link}
            href="/interview/history"
            size="small"
            startIcon={<HistoryIcon />}
            sx={{ color: UI.nightSoft, textTransform: 'none' }}
          >
            面接履歴を見る
          </Button>
        </Box>

        {errorMessage && (
          <Paper sx={{ bgcolor: 'rgba(234,67,53,0.15)', border: '1px solid rgba(234,67,53,0.4)', p: 2, mb: 2, borderRadius: 2 }}>
            <Typography variant="body2" sx={{ color: '#f28b82', mb: finishFailed && onRetryFinish ? 1.5 : 0 }}>{errorMessage}</Typography>
            {finishFailed && onRetryFinish && (
              <Button
                variant="contained"
                onClick={onRetryFinish}
                sx={{ bgcolor: UI.mark, '&:hover': { bgcolor: '#d14f10' } }}
              >
                再試行
              </Button>
            )}
          </Paper>
        )}

        {/* 発話が保存できていないなら、レポートが欠ける・生成されない理由をここで明示する（#1476） */}
        {utteranceSaveFailed && (
          <Paper role="alert" sx={{ bgcolor: 'rgba(251,188,5,0.15)', border: '1px solid rgba(251,188,5,0.4)', p: 2, mb: 2, borderRadius: 2 }}>
            <Typography variant="body2" sx={{ color: '#fdd663' }}>{UTTERANCE_SAVE_FAILED_MESSAGE}</Typography>
          </Paper>
        )}

        {reportStatus === 'pending' && (
          <Paper sx={{ bgcolor: UI.nightPanel, border: '1px solid rgba(255,255,255,0.08)', p: 3, borderRadius: 2 }}>
            {reportProgress ? <ReportProgressCard view={reportProgress} /> : (
              <>
                <Typography sx={{ color: UI.nightInk, mb: 1.5 }}>レポートを生成中です...</Typography>
                <LinearProgress sx={{ bgcolor: '#3c4043', '& .MuiLinearProgress-bar': { bgcolor: '#4285f4' } }} />
              </>
            )}
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
                        sx={{ color: emailSent ? '#34a853' : UI.mark, borderColor: emailSent ? '#34a853' : UI.mark, '&:hover': { borderColor: UI.mark, bgcolor: 'rgba(236,91,19,0.08)' } }}
                      >
                        {emailSent ? '✓ メールを送信しました' : emailSending ? '送信中...' : 'レポートをメールで受け取る'}
                      </Button>
                    </span>
                  </Tooltip>
                  <ErrorAlert error={emailError} sx={{ mb: 0 }} />
                  {isGuest && (
                    <Typography variant="caption" sx={{ color: UI.nightMuted, textAlign: 'center' }}>
                      {GUEST_EMAIL_DISABLED_REASON}{' '}
                      <Button
                        size="small"
                        onClick={onRegisterClick}
                        sx={{ color: UI.mark, textTransform: 'none', p: 0, minWidth: 0, verticalAlign: 'baseline' }}
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
          <Paper sx={{ bgcolor: UI.nightPanel, border: '1px solid rgba(234,67,53,0.3)', p: 3, borderRadius: 2 }}>
            <Typography variant="body2" sx={{ color: '#f28b82', mb: onRetryReport ? 2 : 0 }}>
              {reportFailedByServer
                ? 'レポートを作れませんでした。回答はすべて保存されています。もう一度試すと、最初から作り直します。'
                : reportStatus === 'timeout'
                  ? 'レポート生成がタイムアウトしました。時間をおいて再試行してください。'
                  : 'レポート生成に失敗しました。'}
            </Typography>
            {onRetryReport && (
              <Button
                variant="contained"
                onClick={onRetryReport}
                sx={{ bgcolor: UI.mark, '&:hover': { bgcolor: '#d14f10' } }}
              >
                {reportFailedByServer ? 'もう一度試す' : '再試行'}
              </Button>
            )}
            {/* 再試行そのものが失敗したなら、生成中に戻さずここで知らせる（#1476） */}
            <ErrorAlert error={reportRetryError ?? ''} sx={{ mt: 2, mb: 0 }} />
          </Paper>
        )}

        {/* Video upload status */}
        {videoUploadStatus !== 'idle' && (
          <Paper sx={{ bgcolor: UI.nightPanel, border: '1px solid rgba(255,255,255,0.1)', p: 2, borderRadius: 2 }}>
            {videoSizeWarning && (
              <Typography variant="caption" sx={{ color: '#fdd663', display: 'block', mb: 1 }}>
                ⚠ {videoSizeWarning}
              </Typography>
            )}
            <Typography variant="body2" sx={{ color: videoUploadStatus === 'done' ? '#34a853' : videoUploadStatus === 'error' ? '#f28b82' : UI.nightMuted, mb: videoUploadStatus === 'uploading' ? 1 : 0 }}>
              {videoUploadStatus === 'uploading' && `動画をアップロード中... ${videoUploadProgress}%`}
              {videoUploadStatus === 'done' && '✓ 動画のアップロードが完了しました'}
              {videoUploadStatus === 'error' && '動画のアップロードに失敗しました。ネットワーク接続を確認してください'}
            </Typography>
            {videoUploadStatus === 'uploading' && (
              <LinearProgress
                variant="determinate"
                value={videoUploadProgress}
                sx={{ height: 6, borderRadius: 3, bgcolor: 'rgba(255,255,255,0.1)', '& .MuiLinearProgress-bar': { bgcolor: UI.mark } }}
              />
            )}
          </Paper>
        )}
      </Box>
    </Box>
  )
}

/** 段階リスト＋全体の%。状態は色だけでなくアイコンと文言でも示す */
function ReportProgressCard({ view }: { view: ReportProgressView }) {
  const current = REPORT_STAGE_STEPS.findIndex((s) => s.stage === view.stage)
  return (
    <Stack spacing={2}>
      <Box sx={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between' }}>
        <Typography sx={{ color: UI.nightInk, fontWeight: 700 }}>レポートを作っています</Typography>
        <Typography sx={{ color: UI.mark, fontWeight: 700, fontSize: 26 }}>{view.percent}%</Typography>
      </Box>
      <LinearProgress
        variant="determinate"
        value={view.percent}
        aria-label="レポート作成の進み具合"
        sx={{ height: 8, borderRadius: 4, bgcolor: 'rgba(255,255,255,0.1)', '& .MuiLinearProgress-bar': { bgcolor: UI.mark } }}
      />
      <Stack component="ul" spacing={1} sx={{ listStyle: 'none', m: 0, p: 0 }}>
        {REPORT_STAGE_STEPS.map((step, i) => {
          const done = current < 0 ? view.stage === 'done' : i < current
          const active = i === current
          return (
            <Box component="li" key={step.stage} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
              {done ? <CheckCircleIcon fontSize="small" sx={{ color: '#34a853' }} />
                : active ? <CircularProgress size={18} sx={{ color: UI.mark, mx: '1px' }} />
                : <RadioButtonUncheckedIcon fontSize="small" sx={{ color: UI.nightMuted }} />}
              <Typography variant="body2" sx={{ color: active ? UI.nightInk : UI.nightSoft, flex: 1 }}>{step.label}</Typography>
              <Typography variant="caption" sx={{ color: UI.nightMuted }}>{done ? '完了' : active ? '進行中' : '待機中'}</Typography>
            </Box>
          )
        })}
      </Stack>
      {view.stage === 'generating' && (
        <Typography variant="caption" sx={{ color: UI.nightMuted }}>分析中の%は、進み具合の目安です</Typography>
      )}
      {view.stalled && (
        <Typography role="status" variant="body2" sx={{ color: '#fdd663' }}>
          いつもより時間がかかっています。このまま待つか、あとでもう一度開いてください。回答は保存されています。
        </Typography>
      )}
    </Stack>
  )
}
