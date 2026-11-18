import { Box, Typography } from '@mui/material'
import { HOME_COLORS } from '@/lib/home-theme'

interface ProgressStripProps {
  /** 判定の進み具合（0〜100）。質問数は固定しないので、何問目かは出さない */
  percent: number
}

/** ホームの進捗の帯（Figma「改善G／相談中心」ノード 37:431）。「判定の進み具合 P%」と、答えるとできること。 */
export function ProgressStrip({ percent }: ProgressStripProps) {
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
        {`判定の進み具合 ${percent}%`}
      </Typography>
      <Typography component="span" sx={{ color: HOME_COLORS.soft }}>
        答えると、希望に近い企業が探せます。
      </Typography>
    </Box>
  )
}
