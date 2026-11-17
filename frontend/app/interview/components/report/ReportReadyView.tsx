import { Box, Button, Stack, Tooltip, Typography } from '@mui/material'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import type { InterviewReport } from '@/lib/interview'
import ScoreUpdateBanner, { type WeightScore } from '@/components/feedback/ScoreUpdateBanner'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { GUEST_EMAIL_DISABLED_REASON, GUEST_REGISTER_CTA_LABEL, getGuestEmailButtonProps } from '@/lib/guest-limits'
import { UI } from '@/lib/design/tokens'
import InterviewSummary from '../InterviewSummary'
import { REPORT_BODY_ID } from './ActionButtons'
import { firstSentence } from './resolveReportView'
import { cardSx, secondaryButtonSx } from './reportStyles'

/** 「レポートができました」カード。講評はレポートの要約の一文目だけ（無ければ出さない） */
export function ReadyCard({ report }: { report: InterviewReport }) {
  const comment = firstSentence(report.summary_text)
  return (
    <Box sx={cardSx}>
      <Stack spacing={1}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <CheckCircleOutlineIcon fontSize="small" sx={{ color: 'success.main' }} />
          <Typography component="h2" sx={{ fontWeight: 700 }}>レポートができました</Typography>
        </Box>
        {comment && <Typography variant="body2">{comment}</Typography>}
      </Stack>
    </Box>
  )
}

export type ReportBodyProps = {
  report: InterviewReport
  scoresBefore: WeightScore[] | null
  scoresAfter: WeightScore[] | null
  userId?: number
  emailSending: boolean
  emailSent: boolean
  emailError: string
  onSendEmail: () => void
  /** ゲストはメール送信不可。登録導線を出す */
  isGuest: boolean
  onRegisterClick: () => void
}

/** レポート本体（スコア反映・詳細・メール送信） */
export function ReportBody({ report, scoresBefore, scoresAfter, userId, emailSending, emailSent, emailError, onSendEmail, isGuest, onRegisterClick }: ReportBodyProps) {
  const guestEmail = getGuestEmailButtonProps(isGuest)
  const emailLabel = emailSent ? '✓ メールを送信しました' : emailSending ? '送信中...' : 'レポートをメールで受け取る'
  return (
    <Stack id={REPORT_BODY_ID} spacing={2} sx={{ scrollMarginTop: 16 }}>
      <ScoreUpdateBanner beforeScores={scoresBefore} afterScores={scoresAfter} title="面接結果がプロフィールスコアに反映されました" />
      <InterviewSummary report={report} userId={userId} theme="light" />
      <Stack spacing={1}>
        <Tooltip title={guestEmail.title} disableHoverListener={!guestEmail.disabled}>
          <span>
            <Button variant="outlined" fullWidth disabled={emailSending || emailSent || guestEmail.disabled} onClick={onSendEmail} sx={secondaryButtonSx}>
              {emailLabel}
            </Button>
          </span>
        </Tooltip>
        <ErrorAlert error={emailError} sx={{ mb: 0 }} />
        {isGuest && (
          <Typography variant="caption" sx={{ color: UI.inkSoft, textAlign: 'center' }}>
            {GUEST_EMAIL_DISABLED_REASON}{' '}
            <Button size="small" onClick={onRegisterClick} sx={{ color: UI.mark, textTransform: 'none', p: 0, minWidth: 0, minHeight: 44, verticalAlign: 'baseline' }}>
              {GUEST_REGISTER_CTA_LABEL}
            </Button>
          </Typography>
        )}
      </Stack>
    </Stack>
  )
}
