'use client'

import { Box, Paper, Typography } from '@mui/material'

type ResumeRagReportProps = {
  ragReport: string
  reviewLoading: boolean
}

export function ResumeRagReport({ ragReport, reviewLoading }: ResumeRagReportProps) {
  if (!ragReport) return null

  return (
    <Paper sx={{ p: 3, mt: 4 }} elevation={2}>
      <Typography variant="h5" fontWeight="bold" gutterBottom>
        企業別レビューレポート
        {reviewLoading && (
          <Typography component="span" variant="body2" color="text.secondary" sx={{ ml: 1 }}>
            生成中...
          </Typography>
        )}
      </Typography>
      <Box
        sx={{
          whiteSpace: 'pre-wrap',
          fontFamily: 'inherit',
          fontSize: '0.95rem',
          lineHeight: 1.8,
          color: 'text.primary',
        }}
      >
        {ragReport}
      </Box>
    </Paper>
  )
}
