'use client'

import type { ReactNode } from 'react'
import { Box, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'

type ResumeCardProps = {
  /** 「1　レビューする書類」のような見出し */
  title: string
  children: ReactNode
}

/** 入力画面の面。罫線で囲み、影は使わない（Figma: 履歴書レビュー改善案 Card/書類・Card/応募先）。 */
export function ResumeCard({ title, children }: ResumeCardProps) {
  return (
    <Box
      component="section"
      aria-label={title.replace(/\s+/g, ' ')}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: 2.25,
        p: { xs: 2.5, md: 3.5 },
        bgcolor: UI.field,
        border: `1px solid ${UI.rule}`,
        borderRadius: '8px',
        minWidth: 0,
      }}
    >
      <Typography component="h2" sx={{ fontSize: 20, lineHeight: '28px', fontWeight: 700, color: UI.ink }}>
        {title}
      </Typography>
      {children}
    </Box>
  )
}
