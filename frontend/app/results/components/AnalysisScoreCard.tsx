'use client'

import { Alert, Box, Button, Card, CardContent, Typography } from '@mui/material'
import { Refresh } from '@mui/icons-material'
import type { AnalysisScores } from '../types'

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
    <Box sx={{ maxWidth: 1200, mx: 'auto', p: { xs: 2, sm: 4 }, pb: 0 }}>
      {analysisError && (
        <Alert
          severity="warning"
          sx={{ mb: 2 }}
          action={
            <Button color="inherit" size="small" startIcon={<Refresh />} onClick={onRetryAnalysis}>
              再読み込み
            </Button>
          }
        >
          {analysisError}（4段階分析スコア・向いている職種は表示されません）
        </Alert>
      )}

      {(scoreComment || analysisScores) && (
        <Card elevation={2} sx={{ mb: 2, border: '2px solid', borderColor: 'primary.light', backgroundColor: '#f0f4ff' }}>
          <CardContent>
            <Typography variant="h6" fontWeight="bold" gutterBottom>
              📊 4段階分析スコア
            </Typography>
            {analysisScores && (
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)' }, gap: 2, mb: 2 }}>
                {[
                  { label: '職種分析', value: analysisScores.job },
                  { label: '興味分析', value: analysisScores.interest },
                  { label: '適性分析', value: analysisScores.aptitude },
                  { label: '将来分析', value: analysisScores.future },
                ].map(({ label, value }) => (
                  <Box key={label} sx={{ textAlign: 'center', bgcolor: '#fff', borderRadius: 2, p: 1.5, boxShadow: 1 }}>
                    <Typography variant="caption" color="text.secondary">{label}</Typography>
                    <Typography variant="h5" fontWeight="bold" color="primary.main">{value}%</Typography>
                  </Box>
                ))}
              </Box>
            )}
            {scoreComment && (
              <Typography variant="body2" color="text.secondary">
                {scoreComment}
              </Typography>
            )}
          </CardContent>
        </Card>
      )}
    </Box>
  )
}
