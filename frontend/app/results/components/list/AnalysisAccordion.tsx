'use client'

import { useState } from 'react'
import { Box, Button, Collapse, Typography } from '@mui/material'
import { ExpandMore } from '@mui/icons-material'
import type { SuggestedRole } from '../../types'
import { UI } from '@/lib/design/tokens'
import { AnalysisErrorAlert } from './AnalysisSidePanel'
import type { AnalysisProps } from './AnalysisSidePanel'
import { AxisBars, ContinueChatLink } from './AxisBars'
import { buildAxisItems, isLowAxis } from './axisStatus'
import { panelSx, tint } from './panelStyles'
import { RolesPanel } from './RolesPanel'

interface Props extends AnalysisProps {
  jobSuitabilityComment: string
  suggestedRoles: SuggestedRole[]
}

const BODY_ID = 'analysis-accordion-body'

/** SP 用の開閉カード。閉じていても4軸の値とコメントは見える。 */
export function AnalysisAccordion(props: Props) {
  const { analysisScores, scoreComment, analysisError, onRetryAnalysis, onBack, jobSuitabilityComment, suggestedRoles } = props
  const [open, setOpen] = useState(false)
  const hasRoles = Boolean(jobSuitabilityComment) || suggestedRoles.length > 0
  if (!analysisScores && !scoreComment && !analysisError) return null

  return (
    <Box sx={{ mb: 2 }}>
      <AnalysisErrorAlert analysisError={analysisError} onRetryAnalysis={onRetryAnalysis} />
      {(analysisScores || scoreComment) && (
        <Box sx={panelSx}>
          <Box component="h2" sx={{ m: 0 }}>
            <Box component="button" type="button" aria-expanded={open} aria-controls={BODY_ID} onClick={() => setOpen((v) => !v)}
              sx={{ all: 'unset', boxSizing: 'border-box', width: '100%', minHeight: 44, display: 'flex', alignItems: 'center', justifyContent: 'space-between', cursor: 'pointer', fontSize: 17, fontWeight: 700, color: UI.ink, '&:focus-visible': { outline: `3px solid ${UI.mark}`, outlineOffset: 2 } }}>
              どこまで聞けているか
              <ExpandMore aria-hidden sx={{ transform: open ? 'rotate(180deg)' : 'none', transition: 'transform 150ms' }} />
            </Box>
          </Box>
          {analysisScores && (
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, my: 1 }}>
              {buildAxisItems(analysisScores).map(({ key, label, value }) => {
                const low = isLowAxis(value)
                return (
                  <Box key={key} component="span" sx={{ fontSize: 13, fontWeight: 700, px: 1, py: 0.25, borderRadius: '4px', color: low ? UI.flag : UI.mark, backgroundColor: tint(low ? UI.flag : UI.mark) }}>
                    {label} {value}
                  </Box>
                )
              })}
            </Box>
          )}
          {scoreComment && <Typography sx={{ fontSize: 14, lineHeight: 1.85, color: UI.inkSoft }}>{scoreComment}</Typography>}
          {!open && hasRoles && (
            <Button variant="text" onClick={() => setOpen(true)} sx={{ color: UI.mark, px: 0, minHeight: 44 }}>向いていそうな職種も見る</Button>
          )}
          <Collapse in={open} id={BODY_ID} unmountOnExit>
            <Box sx={{ pt: 1.5 }}>
              {analysisScores && <AxisBars scores={analysisScores} />}
              {analysisScores && <ContinueChatLink scores={analysisScores} onBack={onBack} />}
              <RolesPanel bare jobSuitabilityComment={jobSuitabilityComment} suggestedRoles={suggestedRoles} />
            </Box>
          </Collapse>
        </Box>
      )}
    </Box>
  )
}
