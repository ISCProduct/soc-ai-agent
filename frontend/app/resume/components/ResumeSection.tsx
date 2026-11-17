'use client'

import type { ReactNode } from 'react'
import { Box, Paper, Stack, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'

export type ResumeSectionState = 'todo' | 'active' | 'done'

// 色だけで状態を伝えない。記号と文字を必ず添える。
const STATE_TEXT: Record<ResumeSectionState, string> = {
  todo: '○ これから',
  active: '◐ 作業中',
  done: '✓ 完了',
}

type ResumeSectionProps = {
  step: number
  title: string
  state: ResumeSectionState
  children: ReactNode
}

/** 履歴書レビューの流れ（① 書類 → ② 条件 → ③ 結果）の1ステップ分の面。 */
export function ResumeSection({ step, title, state, children }: ResumeSectionProps) {
  const headingId = `resume-step-${step}`
  const done = state === 'done'
  return (
    <Paper
      component="section"
      aria-labelledby={headingId}
      variant="outlined"
      sx={{ p: { xs: 2, sm: 3 }, mb: 2.5, bgcolor: UI.field, borderColor: UI.rule }}
    >
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{ mb: 2 }}>
        <Box
          aria-hidden
          sx={{
            width: 32,
            height: 32,
            flexShrink: 0,
            borderRadius: '50%',
            display: 'grid',
            placeItems: 'center',
            fontWeight: 700,
            bgcolor: state === 'todo' ? 'transparent' : UI.mark,
            color: state === 'todo' ? UI.inkSoft : 'primary.contrastText',
            border: `2px solid ${state === 'todo' ? UI.rule : UI.mark}`,
          }}
        >
          {done ? '✓' : step}
        </Box>
        <Typography id={headingId} component="h2" variant="h6" sx={{ flexGrow: 1 }}>
          {title}
        </Typography>
        <Typography variant="caption" sx={{ color: UI.inkSoft, whiteSpace: 'nowrap' }}>
          {STATE_TEXT[state]}
        </Typography>
      </Stack>
      {children}
    </Paper>
  )
}
