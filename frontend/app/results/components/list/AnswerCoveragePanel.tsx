'use client'

import { Box, Typography } from '@mui/material'
import { CheckCircle, WarningAmber } from '@mui/icons-material'
import type { AnalysisScores } from '../../types'
import { UI } from '@/lib/design/tokens'
import { AXIS_LEVEL_LABEL, AXIS_LEVEL_LABEL_SHORT, axisLevel, buildAxisItems } from './axisStatus'
import { panelSx } from './panelStyles'

interface Props {
  scores: AnalysisScores
  /** PC は縦のリスト、SP は4つを横に並べる */
  layout: 'list' | 'row'
}

/**
 * 「いまの回答でわかっていること」。職種・興味・適性・将来を、どこまで聞けているかで示す。
 * 色だけで伝えず、字形（✓ ⚠）とことばを必ず添える。
 */
export function AnswerCoveragePanel({ scores, layout }: Props) {
  const items = buildAxisItems(scores)
  return (
    <Box component="section" aria-labelledby="answer-coverage-title" sx={{ ...panelSx, p: { xs: 2.5, md: 2.75 }, pt: { md: 2.5 } }}>
      <Typography id="answer-coverage-title" component="h2" sx={{ fontSize: 17, lineHeight: '24px', fontWeight: 700, color: UI.ink, mb: layout === 'list' ? 1.75 : 2 }}>
        いまの回答でわかっていること
      </Typography>
      <Box
        component="ul"
        sx={{
          listStyle: 'none', m: 0, p: 0, display: layout === 'list' ? 'grid' : 'flex',
          gap: layout === 'list' ? 1.75 : 1, justifyContent: 'space-between',
        }}
      >
        {items.map(({ key, label, value }) => {
          const level = axisLevel(value)
          const good = level === 'good'
          const Icon = good ? CheckCircle : WarningAmber
          const status = (
            <Typography component="span" sx={{ fontSize: layout === 'list' ? 15 : 12, lineHeight: layout === 'list' ? '24px' : 1.5, fontWeight: good ? 400 : 700, color: good ? UI.inkSoft : UI.flag }}>
              {(layout === 'list' ? AXIS_LEVEL_LABEL : AXIS_LEVEL_LABEL_SHORT)[level]}
            </Typography>
          )
          return (
            <Box
              component="li"
              key={key}
              sx={layout === 'list'
                ? { display: 'flex', alignItems: 'center', gap: 1.5 }
                : { flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 0.5, textAlign: 'center' }}
            >
              <Icon aria-hidden sx={{ fontSize: layout === 'list' ? 22 : 24, color: good ? 'success.main' : UI.flag, flexShrink: 0 }} />
              <Typography component="span" sx={{ fontSize: 16, lineHeight: '24px', fontWeight: 700, color: UI.ink, width: layout === 'list' ? 40 : 'auto', flexShrink: 0 }}>
                {label}
              </Typography>
              {status}
            </Box>
          )
        })}
      </Box>
    </Box>
  )
}
