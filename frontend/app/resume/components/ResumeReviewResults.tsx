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
import FormatFindingsSection from '@/components/FormatFindingsSection'

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
      {/*
        スコア無しのときは BE が user_weight_scores へ意図的に何も書かないので、
        反映済みバナーを出さない（#1529）。既存スコア行があるユーザーでは
        差分0件でも「反映されました」と表示され、虚偽の主張になる。
      */}
      {scoresAfter && review.review.score !== null && (
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
        {review.format_findings && (
          <Box sx={{ mb: 3 }}>
            <FormatFindingsSection findings={review.format_findings} showPage />
          </Box>
        )}
        {/* 見出しは format_findings の有無によらず出す。古い応答でも画面の構造を変えない */}
        <Typography variant="h6" fontWeight="bold" gutterBottom>
          改善案
        </Typography>
        <Stack spacing={2}>
          {(review.items ?? []).map((item) => {
            const config = getSeverityConfig(item.severity)
            const quote = item.quote?.trim() ?? ''
            const withheld = item.rewrite_withheld === 'low_ocr_confidence'
            const rewrite = withheld ? '' : (item.rewrite?.trim() ?? '')
            const suggestion = item.suggestion?.trim() ?? ''
            const hasOriginal = quote !== '' || rewrite !== '' || withheld
            return (
              <Card
                key={item.id}
                variant="outlined"
                sx={{ borderLeft: 4, borderLeftColor: config.borderColor }}
              >
                <CardContent>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1, flexWrap: 'wrap' }}>
                    <Chip label={config.label} color={config.color} size="small" />
                    <Typography variant="caption" color="text.secondary">
                      ページ {item.page_number}
                    </Typography>
                  </Box>
                  {quote !== '' && (
                    <Box sx={{ mb: 1.5 }}>
                      <Typography variant="caption" color="text.secondary" component="div">
                        原文
                      </Typography>
                      <Typography variant="body2" sx={{ mt: 0.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                        {quote}
                      </Typography>
                    </Box>
                  )}
                  {withheld ? (
                    <Alert severity="warning" sx={{ mb: 1.5 }}>
                      この行は読み取りの信頼度が低いため、書き換え案は出していません。元の書類を見て、ご自身で書き直してください。
                    </Alert>
                  ) : rewrite !== '' ? (
                    <Box sx={{ mb: 1.5 }}>
                      <Typography variant="caption" color="text.secondary" component="div">
                        書き換え案
                      </Typography>
                      <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                        AIの提案です。事実と違う内容になっていないか、元の文章と見比べてから使ってください。
                      </Typography>
                      <Typography variant="body1" sx={{ mt: 0.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                        {rewrite}
                      </Typography>
                    </Box>
                  ) : null}
                  {item.message && (
                    <Typography variant="body1" fontWeight="medium" sx={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                      {item.message}
                    </Typography>
                  )}
                  {suggestion !== '' && suggestion !== rewrite && (
                    <Typography variant="body2" color="text.secondary" sx={{ mt: 1, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                      {hasOriginal ? `補足: ${suggestion}` : `改善案: ${suggestion}`}
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
