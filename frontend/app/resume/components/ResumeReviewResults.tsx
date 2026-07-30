'use client'

import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import ScoreUpdateBanner, { type WeightScore } from '@/components/ScoreUpdateBanner'
import type { ReviewResult } from '../types'
import { getSeverityConfig } from '../utils'

type ResumeReviewResultsProps = {
  review: ReviewResult | null
  scoresBefore: WeightScore[] | null
  scoresAfter: WeightScore[] | null
  annotateError: string
  onDownload: () => void
}

export function ResumeReviewResults({
  review,
  scoresBefore,
  scoresAfter,
  annotateError,
  onDownload,
}: ResumeReviewResultsProps) {
  if (!review) return null

  return (
    <>
      {scoresAfter && (
        <Box mt={4}>
          <ScoreUpdateBanner
            beforeScores={scoresBefore}
            afterScores={scoresAfter}
            title="職務経歴書レビュー結果がプロフィールスコアに反映されました"
          />
        </Box>
      )}

      <Paper sx={{ p: 3, mt: 4 }} elevation={2}>
        <Typography variant="h5" fontWeight="bold" gutterBottom>
          指摘事項
        </Typography>
        <Box sx={{ mb: 2 }}>
          <Typography variant="h6" gutterBottom>
            総合スコア: {review.review.score} / 100
          </Typography>
          <Typography variant="body1" color="text.secondary">
            {review.review.summary}
          </Typography>
        </Box>
        <Divider sx={{ mb: 3 }} />
        <Stack spacing={2}>
          {(review.items ?? []).map((item) => {
            const config = getSeverityConfig(item.severity)
            return (
              <Card
                key={item.id}
                variant="outlined"
                sx={{ borderLeft: 4, borderLeftColor: config.borderColor }}
              >
                <CardContent>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                    <Chip label={config.label} color={config.color} size="small" />
                    <Typography variant="caption" color="text.secondary">
                      ページ {item.page_number}
                    </Typography>
                  </Box>
                  <Typography variant="body1" fontWeight="medium">
                    {item.message}
                  </Typography>
                  {item.suggestion && (
                    <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                      改善案: {item.suggestion}
                    </Typography>
                  )}
                </CardContent>
              </Card>
            )
          })}
        </Stack>
        <Box sx={{ mt: 3 }}>
          {annotateError && (
            <Alert severity="warning" sx={{ mb: 2 }}>{annotateError}</Alert>
          )}
          {review.annotated_available && (
            <Button variant="outlined" onClick={onDownload}>
              注釈PDFをダウンロード
            </Button>
          )}
        </Box>
      </Paper>
    </>
  )
}
