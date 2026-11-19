'use client'

import { useState } from 'react'
import { Alert, Box, Button, ButtonBase, Collapse, LinearProgress, Stack, Typography } from '@mui/material'
import DownloadIcon from '@mui/icons-material/Download'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { UI } from '@/lib/design/tokens'
import type { ReviewResult } from '../types'
import { FINDING_SEVERITY_LABEL, parseItemScores, RUBRIC_SCORE_MAX, type FindingSeverity } from '../utils'
import { SeverityMark } from './ResumeFindingParts'

type ResumeResultSummaryProps = {
  review: ReviewResult['review']
  counts: Record<FindingSeverity, number>
  annotatedAvailable: boolean
  annotateError: string
  onDownload: () => void
  /** スマホでは下の固定バーに出すので、ここでは出さない */
  showDownload: boolean
}

/** 総合スコアと、重要度ごとの件数。評価の内訳は開いて見る。 */
export function ResumeResultSummary({ review, counts, annotatedAvailable, annotateError, onDownload, showDownload }: ResumeResultSummaryProps) {
  const [open, setOpen] = useState(false)
  // 内訳は item_scores_json から作る。#1529 以前のレビューとスコア無しのレビューは空になる
  const itemScores = parseItemScores(review.item_scores_json)
  const noScore = review.score === null

  return (
    <Box sx={{ mb: 2 }}>
      <Stack
        direction="row"
        alignItems="center"
        sx={{ flexWrap: 'wrap', columnGap: 3, rowGap: 1.5, px: { xs: 2, md: 3 }, py: 2, bgcolor: UI.field, border: `1px solid ${UI.rule}`, borderRadius: '8px' }}
      >
        <Stack direction="row" alignItems="flex-end" spacing={0.5}>
          <Typography sx={{ fontSize: 40, lineHeight: '46px', fontWeight: 700, color: UI.ink }}>{noScore ? '—' : review.score}</Typography>
          {!noScore && <Typography sx={{ fontSize: 15, lineHeight: '36px', color: UI.inkSoft }}>/100</Typography>}
        </Stack>
        <Box>
          <Typography sx={{ fontSize: 12, lineHeight: '18px', fontWeight: 700, color: UI.inkSoft }}>総合スコア</Typography>
          <Typography sx={{ fontSize: 11, lineHeight: '16px', color: UI.inkFaint }}>{noScore ? '算出できませんでした' : 'AIによる目安です'}</Typography>
        </Box>
        <Box aria-hidden sx={{ display: { xs: 'none', md: 'block' }, width: '1px', height: 48, bgcolor: UI.ruleSoft }} />
        <Typography sx={{ flex: '1 1 200px', minWidth: 0, fontSize: 15, lineHeight: '24px', fontWeight: 700, color: UI.ink, overflowWrap: 'anywhere' }}>
          {review.summary}
        </Typography>
        <Stack direction="row" spacing={2} alignItems="center" aria-label="重要度ごとの件数">
          {(['critical', 'warning', 'info'] as const).map((s) => (
            <Stack key={s} direction="row" spacing={0.75} alignItems="center" aria-label={`${FINDING_SEVERITY_LABEL[s]} ${counts[s]}件`}>
              <SeverityMark severity={s} />
              <Typography sx={{ fontSize: 14, color: UI.inkSoft }}>{counts[s]}</Typography>
            </Stack>
          ))}
        </Stack>
        {itemScores.length > 0 && (
          <ButtonBase
            onClick={() => setOpen((v) => !v)}
            aria-expanded={open}
            sx={{ gap: 0.5, minHeight: 44, fontSize: 14, fontWeight: 700, color: UI.mark, textDecoration: 'underline', textUnderlinePosition: 'from-font', borderRadius: '2px' }}
          >
            評価の内訳
            <ExpandMoreIcon sx={{ fontSize: 18, transform: open ? 'rotate(180deg)' : 'none' }} aria-hidden />
          </ButtonBase>
        )}
        {showDownload && annotatedAvailable && (
          <Button
            variant="outlined"
            onClick={onDownload}
            startIcon={<DownloadIcon sx={{ fontSize: 18 }} />}
            sx={{ color: UI.ink, borderColor: UI.inkFaint, borderWidth: 1.5, fontSize: 14, '&:hover': { borderWidth: 1.5, borderColor: UI.ink, bgcolor: UI.field } }}
          >
            注釈つきPDFをダウンロード
          </Button>
        )}
      </Stack>

      <Collapse in={open && itemScores.length > 0} unmountOnExit>
        <Stack spacing={1.5} sx={{ mt: 1, p: 2.5, bgcolor: UI.field, border: `1px solid ${UI.rule}`, borderRadius: '8px' }}>
          {itemScores.map((item) => (
            <Box key={item.key}>
              <Stack direction="row" justifyContent="space-between" sx={{ mb: 0.5 }}>
                <Typography sx={{ fontSize: 14, color: UI.inkSoft }}>{item.label}</Typography>
                <Typography sx={{ fontSize: 14, fontWeight: 700, color: UI.ink }}>
                  {item.score} / {RUBRIC_SCORE_MAX}
                </Typography>
              </Stack>
              <LinearProgress
                variant="determinate"
                value={(item.score / RUBRIC_SCORE_MAX) * 100}
                aria-label={`${item.label} ${item.score} / ${RUBRIC_SCORE_MAX}`}
                sx={{ height: 6, borderRadius: 3 }}
              />
            </Box>
          ))}
        </Stack>
      </Collapse>

      {noScore && (
        // スコア無し（#1529）。0点と書くと最低評価に見えるため、算出できなかったことを伝える
        <Alert severity="info" sx={{ mt: 1 }}>
          総合スコアを算出できませんでした。下の指摘事項はご利用いただけます。もう一度レビューを実行するとスコアが付く場合があります。
        </Alert>
      )}
      {annotateError && (
        <Alert severity="warning" sx={{ mt: 1 }}>
          {annotateError}
        </Alert>
      )}
    </Box>
  )
}
