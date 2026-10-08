import { Box } from '@mui/material'
import type { MarkTone } from '@/lib/company/marks'

const TONE_COLOR: Record<MarkTone, string> = {
  neutral: 'text.primary',
  attention: 'warning.main',
  progress: 'primary.main',
  stop: 'error.main',
}

export function MarkLabel({
  mark,
  label,
  tone,
}: {
  mark: string
  label: string
  tone: MarkTone
}) {
  return (
    <Box
      component="span"
      sx={{ color: TONE_COLOR[tone], fontWeight: 700, whiteSpace: 'nowrap' }}
    >
      <Box component="span" aria-hidden="true">
        {mark}{' '}
      </Box>
      {label}
    </Box>
  )
}
