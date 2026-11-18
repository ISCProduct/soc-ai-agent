'use client'

import { Box, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'

type Option<T extends string> = { value: T; label: string }

type ResumeSegmentedProps<T extends string> = {
  label: string
  options: Option<T>[]
  value: T
  onChange: (value: T) => void
}

/** 2択の切り替え（応募先の決め方・候補者区分）。選択中は白い面と太字で示す。 */
export function ResumeSegmented<T extends string>({ label, options, value, onChange }: ResumeSegmentedProps<T>) {
  return (
    <Box
      role="radiogroup"
      aria-label={label}
      sx={{ display: 'flex', gap: 0.5, p: 0.5, bgcolor: UI.tint, borderRadius: '8px', width: '100%' }}
    >
      {options.map((o) => {
        const selected = o.value === value
        return (
          <Box
            key={o.value}
            component="button"
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(o.value)}
            sx={{
              flex: 1,
              minHeight: 44,
              px: 1.5,
              py: '9px',
              cursor: 'pointer',
              borderRadius: '6px',
              border: selected ? `1px solid ${UI.rule}` : '1px solid transparent',
              bgcolor: selected ? UI.field : 'transparent',
              color: selected ? UI.ink : UI.inkSoft,
              font: 'inherit',
            }}
          >
            <Typography component="span" sx={{ fontSize: 15, fontWeight: selected ? 700 : 400, color: 'inherit' }}>
              {o.label}
            </Typography>
          </Box>
        )
      })}
    </Box>
  )
}
