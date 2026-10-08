'use client'

import { Box, Button } from '@mui/material'
import { PIPELINE_STAGES, applicationMark } from '@/lib/company/marks'

export function PipelineBand({
  counts,
  selected,
  onSelect,
}: {
  counts: Record<string, number>
  selected: string
  onSelect: (status: string) => void
}) {
  return (
    <Box
      role="group"
      aria-label="選考の段階"
      sx={{
        display: 'flex',
        flexWrap: 'wrap',
        gap: 1,
        mb: 2,
        pb: 1,
      }}
    >
      {PIPELINE_STAGES.map((status) => {
        const mark = applicationMark(status)
        const count = counts[status] ?? 0
        const active = selected === status
        return (
          <Button
            key={status}
            size="small"
            variant="outlined"
            color={mark.tone === 'stop' ? 'error' : mark.tone === 'attention' ? 'warning' : 'primary'}
            aria-pressed={active}
            onClick={() => onSelect(active ? '' : status)}
            sx={{ fontWeight: active ? 700 : 500, borderWidth: active ? 2 : 1 }}
          >
            <Box component="span" aria-hidden="true" sx={{ mr: 0.5 }}>
              {mark.mark}
            </Box>
            {mark.label} {count}
          </Button>
        )
      })}
    </Box>
  )
}

export function pipelineTotal(counts: Record<string, number> | undefined): number {
  if (!counts) return 0
  return Object.values(counts).reduce((sum, n) => sum + n, 0)
}
