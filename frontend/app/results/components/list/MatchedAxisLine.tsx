'use client'

import { Box, Typography } from '@mui/material'
import type { CategoryScores } from '../../types'
import { getTopCategoryScores } from '../../utils'
import { UI } from '@/lib/design/tokens'

interface Props {
  categoryScores?: CategoryScores
}

/** 「一致している項目」と、上位3軸の名前と点数。categoryScores が無ければ出さない。 */
export function MatchedAxisLine({ categoryScores }: Props) {
  if (!categoryScores) return null
  return (
    <Box sx={{ display: 'flex', flexDirection: { xs: 'column', md: 'row' }, alignItems: { md: 'center' }, columnGap: 1.5, rowGap: 0.25 }}>
      <Typography component="span" sx={{ fontSize: 13, lineHeight: '22px', color: UI.inkSoft }}>一致している項目</Typography>
      <Typography component="span" sx={{ fontSize: 15, lineHeight: '22px', fontWeight: 700, color: UI.ink }}>
        {getTopCategoryScores(categoryScores, 3).map(({ label, score }) => `${label} ${Math.round(score)}`).join('　')}
      </Typography>
    </Box>
  )
}
