'use client'

import { Alert, Box, Button, Typography } from '@mui/material'
import { Refresh } from '@mui/icons-material'
import type { AnalysisScores } from '../../types'
import { UI } from '@/lib/design/tokens'
import { AxisBars, ContinueChatLink } from './AxisBars'
import { panelSx } from './panelStyles'

export interface AnalysisProps {
  analysisScores: AnalysisScores | null
  scoreComment: string
  analysisError: string | null
  onRetryAnalysis: () => void
  onBack: () => void
}

/** 分析取得失敗の警告（PC サイド・SP 共通）。 */
export function AnalysisErrorAlert({ analysisError, onRetryAnalysis }: Pick<AnalysisProps, 'analysisError' | 'onRetryAnalysis'>) {
  if (!analysisError) return null
  return (
    <Alert severity="warning" sx={{ mb: 2 }}
      action={<Button color="inherit" size="small" startIcon={<Refresh />} onClick={onRetryAnalysis}>再読み込み</Button>}>
      {analysisError}（4段階分析スコア・向いている職種は表示されません）
    </Alert>
  )
}

/** PC 右サイドの「どこまで聞けているか」パネル。 */
export function AnalysisSidePanel(props: AnalysisProps) {
  const { analysisScores, scoreComment, analysisError, onRetryAnalysis, onBack } = props
  if (!analysisScores && !scoreComment && !analysisError) return null
  return (
    <Box>
      <AnalysisErrorAlert analysisError={analysisError} onRetryAnalysis={onRetryAnalysis} />
      {(analysisScores || scoreComment) && (
        <Box sx={panelSx}>
          <Typography component="h2" sx={{ fontSize: 17, fontWeight: 700, color: UI.ink, mb: 1.5 }}>どこまで聞けているか</Typography>
          {analysisScores && <AxisBars scores={analysisScores} />}
          {scoreComment && <Typography sx={{ fontSize: 14, lineHeight: 1.85, color: UI.inkSoft }}>{scoreComment}</Typography>}
          {analysisScores && <ContinueChatLink scores={analysisScores} onBack={onBack} />}
        </Box>
      )}
    </Box>
  )
}
