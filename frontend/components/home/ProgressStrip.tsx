import { Box, Typography } from '@mui/material'
import { HOME_COLORS } from '@/lib/home-theme'

interface ProgressStripProps {
  /** 有効に答えた質問数 */
  valid: number
  /** 想定の質問数（既定は 15。API の total_questions に寄せる） */
  required: number
  percent: number
}

/** ホームの進捗の帯（Figma「改善G／相談中心」ノード 37:431）。「質問 N / M 問（P%）」と、答えるとできること。 */
export function ProgressStrip({ valid, required, percent }: ProgressStripProps) {
  return (
    <Box
      component="section"
      aria-label="診断の進み具合"
      sx={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        columnGap: 1.5,
        bgcolor: HOME_COLORS.white,
        border: `1px solid ${HOME_COLORS.border}`,
        borderRadius: '8px',
        p: 2,
        fontSize: 16,
        lineHeight: 1.7,
      }}
    >
      <Typography component="span" sx={{ fontWeight: 700, color: HOME_COLORS.ink }}>
        {`質問 ${valid} / ${required} 問（${percent}%）`}
      </Typography>
      <Typography component="span" sx={{ color: HOME_COLORS.soft }}>
        答えると、希望に近い企業が探せます。
      </Typography>
    </Box>
  )
}
