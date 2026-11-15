'use client'

import { Box, Button, Typography } from '@mui/material'
import { ArrowForward } from '@mui/icons-material'
import type { AnalysisScores } from '../../types'
import { UI } from '@/lib/design/tokens'
import { barWidthPercent, buildAxisItems, findLowestLowAxis, isLowAxis } from './axisStatus'

/** 4軸の横棒。50未満は色と数値の両方で示す。 */
export function AxisBars({ scores }: { scores: AnalysisScores }) {
  return (
    <Box sx={{ display: 'grid', gap: 1, mb: 2 }}>
      {buildAxisItems(scores).map(({ key, label, value }) => {
        const low = isLowAxis(value)
        return (
          <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Typography sx={{ fontSize: 13, color: UI.inkSoft, width: 32, flexShrink: 0 }}>{label}</Typography>
            <Box sx={{ flex: 1, maxWidth: 204, height: 8, backgroundColor: UI.ruleSoft, borderRadius: 1, overflow: 'hidden' }}>
              <Box sx={{ width: `${barWidthPercent(value)}%`, height: '100%', backgroundColor: low ? UI.flag : UI.mark }} />
            </Box>
            <Typography sx={{ fontSize: 13, width: 30, textAlign: 'right', fontVariantNumeric: 'tabular-nums', color: low ? UI.flag : UI.inkSoft, fontWeight: low ? 700 : 400 }}>
              {value}
            </Typography>
          </Box>
        )
      })}
    </Box>
  )
}

/** 最も低い軸（50未満）がある時だけ、チャットへ戻る導線を出す。 */
export function ContinueChatLink({ scores, onBack }: { scores: AnalysisScores; onBack: () => void }) {
  const lowest = findLowestLowAxis(buildAxisItems(scores))
  if (!lowest) return null
  return (
    <Button variant="text" endIcon={<ArrowForward />} onClick={onBack} sx={{ color: UI.mark, px: 0, mt: 1, minHeight: 44, justifyContent: 'flex-start' }}>
      チャットで{lowest.label}の話を続ける
    </Button>
  )
}
