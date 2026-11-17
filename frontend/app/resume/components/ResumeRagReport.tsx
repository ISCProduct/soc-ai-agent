'use client'

import { Box, Stack, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'

type ResumeRagReportProps = {
  ragReport: string
  reviewLoading: boolean
  interrupted: boolean
}

export function ResumeRagReport({ ragReport, reviewLoading, interrupted }: ResumeRagReportProps) {
  if (!ragReport) return null

  // 色だけで伝えず、記号と文で状態を示す。途中で止まったものは完成版と区別する
  const status = reviewLoading ? '◐ 生成中' : interrupted ? '⚠ 途中まで（未完成）' : '✓ 生成済み'

  return (
    <Box
      component="article"
      aria-labelledby="resume-rag-report-title"
      sx={{ p: { xs: 2, sm: 2.5 }, border: `1px solid ${UI.rule}`, borderRadius: 1, bgcolor: UI.paper }}
    >
      <Stack direction="row" alignItems="baseline" justifyContent="space-between" spacing={1} sx={{ mb: 1 }}>
        <Typography id="resume-rag-report-title" component="h3" variant="h6">
          企業別レビューレポート
        </Typography>
        <Typography variant="caption" sx={{ color: interrupted ? UI.flag : UI.inkSoft, fontWeight: 700, whiteSpace: 'nowrap' }}>
          {status}
        </Typography>
      </Stack>
      <Box sx={{ whiteSpace: 'pre-wrap', fontFamily: 'inherit', lineHeight: 1.8, color: UI.ink }}>{ragReport}</Box>
      <Typography variant="caption" sx={{ display: 'block', mt: 1.5, color: UI.inkSoft }}>
        AIが生成した内容です。内容を確認し、ご自身の判断で書類に反映してください。
      </Typography>
    </Box>
  )
}
