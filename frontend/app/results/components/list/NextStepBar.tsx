'use client'

import { Box, Button, Typography } from '@mui/material'
import type { AnalysisScores } from '../../types'
import { CONTENT_MAX_WIDTH, UI } from '@/lib/design/tokens'
import { buildAxisItems, buildNextStep, findWeakestAxis } from './axisStatus'

interface Props {
  analysisScores: AnalysisScores | null
  isProvisional: boolean
  onBack: () => void
}

/**
 * 「次にやること」の帯。いちばん聞けていない軸を挙げて、チャットで続ける導線を出す。
 * 分析を取得できていない（軸の値が無い）ときは、暫定評価でなければ出さない。
 */
export function NextStepBar({ analysisScores, isProvisional, onBack }: Props) {
  if (!analysisScores && !isProvisional) return null
  const weakest = analysisScores ? findWeakestAxis(buildAxisItems(analysisScores)) : null
  const { message, showChatAction } = buildNextStep(weakest, isProvisional)

  return (
    <Box component="section" aria-label="次にやること" sx={{ backgroundColor: UI.tint, borderBottom: `1px solid ${UI.rule}`, flexShrink: 0, px: { xs: 2, md: 4 }, py: { xs: 2, md: 2.25 } }}>
      <Box
        sx={{
          maxWidth: CONTENT_MAX_WIDTH, mx: 'auto', display: 'flex', gap: { xs: 1.5, md: 3 },
          flexDirection: { xs: 'column', md: 'row' }, alignItems: { xs: 'stretch', md: 'center' }, justifyContent: 'space-between',
        }}
      >
        <Box>
          <Typography sx={{ fontSize: 13, fontWeight: 700, color: UI.inkSoft, lineHeight: '18px' }}>次にやること</Typography>
          <Typography sx={{ fontSize: { xs: 16, md: 17 }, fontWeight: 700, color: UI.ink, lineHeight: { xs: 1.6, md: '26px' } }}>{message}</Typography>
        </Box>
        {showChatAction && (
          <Button variant="contained" onClick={onBack} sx={{ backgroundColor: UI.mark, boxShadow: 'none', fontSize: 15, minHeight: 44, px: 2.75, flexShrink: 0, whiteSpace: 'nowrap' }}>
            チャットで続きを話す
          </Button>
        )}
      </Box>
    </Box>
  )
}
