'use client'

import { Button } from '@mui/material'
import { Alert } from '@/components/common/Alert'
import { Refresh } from '@mui/icons-material'

interface Props {
  analysisError: string | null
  onRetryAnalysis: () => void
}

/** 分析取得失敗の警告（PC・SP 共通）。 */
export function AnalysisErrorAlert({ analysisError, onRetryAnalysis }: Props) {
  if (!analysisError) return null
  return (
    <Alert role="alert" severity="warning" sx={{ mb: 2 }}
      action={<Button color="inherit" size="small" startIcon={<Refresh />} onClick={onRetryAnalysis}>再読み込み</Button>}>
      {analysisError}（回答の分析・向いている職種は表示されません）
    </Alert>
  )
}
