'use client'

import { Box, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'

/** 適合度。数字は大きく、「適合度」は小さく添える。 */
export function FitScore({ score }: { score: number }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 0.75, whiteSpace: 'nowrap' }}>
      <Typography component="span" sx={{ fontSize: 12, color: UI.inkSoft }}>適合度</Typography>
      <Typography component="span" sx={{ fontSize: 24, lineHeight: '28px', fontWeight: 700, color: UI.ink, fontVariantNumeric: 'tabular-nums' }}>{score}</Typography>
    </Box>
  )
}
