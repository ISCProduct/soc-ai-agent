'use client'

import { Box } from '@mui/material'
import type { AnalysisScores } from '../types'
import { AnalysisErrorAlert } from './list/AnalysisErrorAlert'
import { AnswerCoveragePanel } from './list/AnswerCoveragePanel'

export interface AnalysisScoreCardProps {
  analysisScores: AnalysisScores | null
  /** 一覧では出さなくなった（Figma v4）。エラー・0件の画面から渡される互換のために残す */
  scoreComment?: string
  analysisError: string | null
  onRetryAnalysis: () => void
}

/**
 * エラー・0件の画面に出す「いまの回答でわかっていること」。
 * 一覧と同じ部品なので、見た目・文言・判定が一覧と食い違わない。
 */
export default function AnalysisScoreCard({ analysisScores, analysisError, onRetryAnalysis }: AnalysisScoreCardProps) {
  if (!analysisScores && !analysisError) return null

  return (
    <>
      <AnalysisErrorAlert analysisError={analysisError} onRetryAnalysis={onRetryAnalysis} />
      {analysisScores && (
        <Box sx={{ mb: 4, maxWidth: 340 }}>
          <AnswerCoveragePanel scores={analysisScores} layout="list" />
        </Box>
      )}
    </>
  )
}
