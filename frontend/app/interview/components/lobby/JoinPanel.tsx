import { Box, Button, Stack, Typography } from '@mui/material'
import { interviewLimits } from '@/lib/interview'
import { UI } from '@/lib/design/tokens'
import type { InterviewCompany } from '../../types'
import { CompanyInfo, MatchingBanner, UnregisteredWarning } from './CompanyNotices'
import ConsentCheck from './ConsentCheck'
import FocusGoalCard, { type FocusGoal } from './FocusGoalCard'
import { MIN_TAP_SIZE, PANEL_WIDTH, textLinkSx } from './lobbyStyles'

interface JoinPanelProps {
  companyName: string
  positionTitle: string
  totalQuestionCount: number
  interviewCompany: InterviewCompany | null
  fromMatchingResults: boolean
  focusGoal?: FocusGoal | null
  consentGiven: boolean
  onConsentChange: (checked: boolean) => void
  onJoin: () => void
  onBack: () => void
}

/** 右側の操作列。見出し → 概要 → 注意 → 同意 → 開始ボタン の順に縦に並べる。 */
export default function JoinPanel({
  companyName,
  positionTitle,
  totalQuestionCount,
  interviewCompany,
  fromMatchingResults,
  focusGoal,
  consentGiven,
  onConsentChange,
  onJoin,
  onBack,
}: JoinPanelProps) {
  const isUnregistered = interviewCompany?.id === 0 && Boolean(interviewCompany.name.trim())
  return (
    <Stack spacing={2} sx={{ width: '100%', maxWidth: { md: PANEL_WIDTH } }}>
      {fromMatchingResults && <MatchingBanner company={interviewCompany} />}
      <Box>
        <Typography component="h1" sx={{ fontSize: { xs: 24, md: 28 }, fontWeight: 700, color: UI.ink, mb: 0.5 }}>
          面接の準備ができました
        </Typography>
        <Typography sx={{ fontWeight: 700, color: UI.ink }}>
          {companyName}
          {positionTitle && ` ／ ${positionTitle}`}
        </Typography>
        <Typography sx={{ fontSize: 13, color: UI.inkSoft }}>
          最大{interviewLimits.maxMinutes}分・質問{totalQuestionCount}問
        </Typography>
      </Box>
      {isUnregistered && <UnregisteredWarning />}
      <FocusGoalCard goal={focusGoal} />
      <ConsentCheck checked={consentGiven} onChange={onConsentChange} />
      <Button
        variant="contained"
        disabled={!consentGiven}
        onClick={onJoin}
        sx={{
          minHeight: 56,
          borderRadius: 9999,
          fontSize: 16,
          fontWeight: 700,
          textTransform: 'none',
          bgcolor: UI.mark,
          color: UI.field,
          boxShadow: 'none',
          '&:hover': { bgcolor: UI.mark, opacity: 0.9, boxShadow: 'none' },
          '&.Mui-disabled': { bgcolor: UI.ruleSoft, color: UI.inkFaint },
        }}
      >
        面接を始める
      </Button>
      {!consentGiven && (
        <Typography sx={{ fontSize: 12, color: UI.inkSoft, textAlign: 'center' }}>
          同意にチェックを入れると始められます
        </Typography>
      )}
      <Button onClick={onBack} sx={{ ...textLinkSx, alignSelf: 'center', minHeight: MIN_TAP_SIZE }}>
        練習内容を変える
      </Button>
      {interviewCompany && <CompanyInfo company={interviewCompany} />}
    </Stack>
  )
}
