'use client'

import { Alert, Box, Button, Typography } from '@mui/material'
import { Refresh } from '@mui/icons-material'
import type { AnalysisScores } from '../types'
import { SHORTLIST } from '../shortlistTokens'

export interface AnalysisScoreCardProps {
  analysisScores: AnalysisScores | null
  scoreComment: string
  analysisError: string | null
  onRetryAnalysis: () => void
}

export default function AnalysisScoreCard({
  analysisScores,
  scoreComment,
  analysisError,
  onRetryAnalysis,
}: AnalysisScoreCardProps) {
  if (!scoreComment && !analysisScores && !analysisError) return null

  return (
    <>
      {analysisError && (
        <Alert
          severity="warning"
          sx={{ mb: 3 }}
          action={
            <Button color="inherit" size="small" startIcon={<Refresh />} onClick={onRetryAnalysis}>
              再読み込み
            </Button>
          }
        >
          {analysisError}（4段階分析スコア・向いている職種は表示されません）
        </Alert>
      )}

      {/*
        4つの大きな%をやめた。
        100% / 100% / 55% / 20% と並んでも、学生が次に何をすればいいかは分からない。
        どこまで聞けたかの進み具合なので、横一列の目盛りで「まだ将来の話が薄い」と
        一目で分かる形にする。数字は補助に落とす。
      */}
      {(scoreComment || analysisScores) && (
        <Box sx={{ mb: 4 }}>
          <Typography sx={{ fontSize: 17, fontWeight: 700, color: SHORTLIST.ink, mb: 1.5 }}>
            どこまで聞けているか
          </Typography>
          {analysisScores && (
            <Box sx={{ display: 'grid', gap: 1, mb: 2, maxWidth: 560 }}>
              {[
                { label: '職種', value: analysisScores.job },
                { label: '興味', value: analysisScores.interest },
                { label: '適性', value: analysisScores.aptitude },
                { label: '将来', value: analysisScores.future },
              ].map(({ label, value }) => (
                <Box key={label} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                  <Typography sx={{ fontSize: 14, color: SHORTLIST.inkSoft, minWidth: 40 }}>{label}</Typography>
                  <Box sx={{ flex: 1, height: 8, backgroundColor: SHORTLIST.ruleSoft, position: 'relative' }}>
                    <Box sx={{
                      position: 'absolute', inset: 0, right: 'auto',
                      width: `${Math.max(0, Math.min(100, value))}%`,
                      backgroundColor: value < 50 ? SHORTLIST.flag : SHORTLIST.mark,
                    }} />
                  </Box>
                  <Typography sx={{
                    fontSize: 13, color: SHORTLIST.inkSoft, minWidth: 34, textAlign: 'right',
                    fontVariantNumeric: 'tabular-nums',
                  }}>
                    {value}
                  </Typography>
                </Box>
              ))}
            </Box>
          )}
          {scoreComment && (
            <Typography sx={{ fontSize: 14.5, lineHeight: 1.9, color: SHORTLIST.inkSoft, maxWidth: '64ch' }}>
              {scoreComment}
            </Typography>
          )}
        </Box>
      )}
    </>
  )
}
