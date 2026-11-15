'use client'

import { Box, Typography } from '@mui/material'
import type { CategoryScores } from '../../types'
import { getTopCategoryScores } from '../../utils'
import { UI } from '@/lib/design/tokens'
import { tint } from './panelStyles'

const chipSx = { fontSize: 13, fontWeight: 700, px: 1, py: 0.25, borderRadius: '4px', lineHeight: 1.6 } as const

interface Props {
  categoryScores?: CategoryScores
  /** 他の候補より根拠が薄いときの軸数。null なら印を出さない */
  thinAxisCount: number | null
}

/** 「噛み合った軸」ラベルと上位3軸のチップ。categoryScores が無ければ出さない。 */
export function MatchedAxisChips({ categoryScores, thinAxisCount }: Props) {
  if (!categoryScores) return null
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1, mb: 2 }}>
      <Typography component="span" sx={{ fontSize: 13, color: UI.inkSoft }}>噛み合った軸</Typography>
      {getTopCategoryScores(categoryScores, 3).map(({ label, score }) => (
        <Box key={label} component="span" sx={{ ...chipSx, color: UI.mark, backgroundColor: tint(UI.mark) }}>
          {label} {Math.round(score)}
        </Box>
      ))}
      {thinAxisCount !== null && (
        <Box component="span" sx={{ ...chipSx, color: UI.flag, backgroundColor: tint(UI.flag) }}>
          根拠 {thinAxisCount}軸（他の候補より少なめ）
        </Box>
      )}
    </Box>
  )
}
