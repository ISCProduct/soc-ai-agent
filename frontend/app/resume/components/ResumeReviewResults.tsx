'use client'

import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  LinearProgress,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import ScoreUpdateBanner, { type WeightScore } from '@/components/ScoreUpdateBanner'
import type { ReviewResult } from '../types'
import { getSeverityConfig, parseItemScores, RUBRIC_SCORE_MAX } from '../utils'

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

  // 内訳は item_scores_json から作る。#1529 以前のレビューとスコア無しのレビューは空になる。
  const itemScores = parseItemScores(review.review.item_scores_json)

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
          {review.review.score === null ? (
            // スコア無し（#1529）。0点と書くと最低評価に見えるため、算出できなかったことを伝える。
            <Alert severity="info" sx={{ mb: 1 }}>
              総合スコアを算出できませんでした。下の指摘事項はご利用いただけます。もう一度レビューを実行するとスコアが付く場合があります。
            </Alert>
          ) : (
            <Typography variant="h6" gutterBottom>
              総合スコア: {review.review.score} / 100
            </Typography>
          )}
          <Typography variant="body1" color="text.secondary">
            {review.review.summary}
          </Typography>
        </Box>
        {itemScores.length > 0 && (
          <Box sx={{ mb: 2 }}>
            <Typography variant="subtitle1" fontWeight="bold" gutterBottom>
              評価の内訳
            </Typography>
            <Stack spacing={1.5}>
              {itemScores.map((item) => (
                <Box key={item.key}>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 0.5 }}>
                    <Typography variant="body2" color="text.secondary">
                      {item.label}
                    </Typography>
                    <Typography variant="body2" fontWeight="bold">
                      {item.score} / {RUBRIC_SCORE_MAX}
                    </Typography>
                  </Box>
                  <LinearProgress
                    variant="determinate"
                    value={(item.score / RUBRIC_SCORE_MAX) * 100}
                    aria-label={`${item.label} ${item.score} / ${RUBRIC_SCORE_MAX}`}
                    sx={{ height: 6, borderRadius: 3 }}
                  />
                </Box>
              ))}
            </Stack>
          </Box>
        )}
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
