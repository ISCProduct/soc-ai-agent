import { Alert, Box, Button, Chip, Divider, IconButton, Paper, Stack, Tooltip, Typography } from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import CheckIcon from '@mui/icons-material/Check'
import FormatFindingsSection from '@/components/feedback/FormatFindingsSection'
import { UI } from '@/lib/design/tokens'
import type { createTextDiff } from '../text-diff'
import type { AnchoredFeedbackItem, ReviewFeedbackItem, ReviewResult } from '../types'
import CharCountNote from './CharCountNote'
import HighlightedDiffText from './HighlightedDiffText'
import ReviewScoreCard from './ReviewScoreCard'

const FEEDBACK_SCORE_LABELS: Record<ReviewFeedbackItem['score_key'], string> = {
  specificity_score: '具体性',
  star_score: 'STAR法準拠',
  company_fit_score: '企業適合性',
  length_balance_score: '文字数バランス',
}

type Props = {
  reviewResult: ReviewResult
  // 添削に使った原文・企業名・字数上限。入力欄はあとから編集できるため結果と一緒に受け取る
  reviewedOriginalText: string
  reviewedCompany: string
  requestedCharLimit: number | null
  // 差分と指摘アンカーは呼び出し側で useMemo する（HighlightedDiffText の memo を効かせるため）
  reviewDiff: ReturnType<typeof createTextDiff> | null
  anchoredFeedback: AnchoredFeedbackItem[]
  copied: boolean
  onCopy: (text: string) => void
}

// ES添削の結果（スコア・形式チェック・フィードバック・企業別アドバイス・改善前後の差分）
export default function ReviewResultPanel({
  reviewResult,
  reviewedOriginalText,
  reviewedCompany,
  requestedCharLimit,
  reviewDiff,
  anchoredFeedback,
  copied,
  onCopy,
}: Props) {
  return (
    <Stack spacing={2}>
      {/* Score gauges */}
        <ReviewScoreCard reviewResult={reviewResult} reviewedCompany={reviewedCompany} />

      {/* Feedback */}
      {reviewResult.format_findings && (
        <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `1px solid ${UI.ruleSoft}`, bgcolor: UI.field }}>
          <FormatFindingsSection findings={reviewResult.format_findings} text={reviewedOriginalText} />
        </Paper>
      )}

      <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `1px solid ${UI.ruleSoft}`, bgcolor: UI.field }}>
        <Typography sx={{ fontWeight: 700, fontSize: 16, mb: 1.5 }}>フィードバック</Typography>
        <Typography variant="body2" sx={{ color: UI.inkSoft, lineHeight: 1.8 }}>
          {reviewResult.feedback}
        </Typography>
        {anchoredFeedback.length > 0 && (
          <Box sx={{ mt: 2 }}>
            <Typography sx={{ fontWeight: 700, fontSize: 14, mb: 1 }}>
              指摘と改善のヒント
            </Typography>
            <Stack spacing={1.5}>
              {anchoredFeedback.map(item => (
                <Box
                  key={item.index}
                  sx={{ borderLeft: `3px solid ${UI.mark}`, pl: 1.5, py: 0.25 }}
                >
                  <Typography sx={{ fontSize: 12, fontWeight: 700, color: UI.mark, mb: 0.5 }}>
                    {FEEDBACK_SCORE_LABELS[item.score_key]}
                  </Typography>
                  <Typography variant="body2" sx={{ color: UI.ink, lineHeight: 1.7 }}>
                    {item.issue}
                  </Typography>
                  <Typography
                    component="blockquote"
                    sx={{
                      m: 0,
                      mt: 0.75,
                      pl: 1,
                      borderLeft: `2px solid ${UI.ruleSoft}`,
                      color: UI.inkFaint,
                      fontSize: 13,
                    }}
                  >
                    「{item.source_excerpt}」
                  </Typography>
                  <Typography variant="body2" sx={{ color: UI.inkSoft, lineHeight: 1.7, mt: 0.5 }}>
                    改善のヒント：{item.suggestion}
                  </Typography>
                  <Button
                    component="a"
                    href={`#${item.anchorId}`}
                    size="small"
                    onClick={() => document.getElementById(item.anchorId)?.focus({ preventScroll: true })}
                    sx={{ mt: 0.5, px: 0, minWidth: 0, textTransform: 'none' }}
                  >
                    原文の該当箇所を見る
                  </Button>
                </Box>
              ))}
            </Stack>
          </Box>
        )}
      </Paper>

      {reviewResult.company_strategy && (
        <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `1px solid ${UI.tint}`, bgcolor: UI.field }}>
          <Typography sx={{ fontWeight: 700, fontSize: 16, mb: 1.5 }}>🏢 {reviewedCompany || '志望企業'}への対策アドバイス</Typography>
          <Typography variant="body2" sx={{ color: UI.inkSoft, lineHeight: 1.8, whiteSpace: 'pre-wrap' }}>{reviewResult.company_strategy}</Typography>
        </Paper>
      )}

      {/* Before / After comparison */}
      {(reviewResult.improved_text || reviewResult.improved_length_satisfied === false) && (
        <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `1px solid ${UI.ruleSoft}`, bgcolor: UI.field }}>
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
            <Typography sx={{ fontWeight: 700, fontSize: 16 }}>改善後テキスト</Typography>
            {reviewResult.improved_text && (
              <Tooltip title={copied ? 'コピーしました' : 'クリップボードにコピー'}>
                <IconButton
                  size="small"
                  onClick={() => onCopy(reviewResult.improved_text)}
                  aria-label="改善文をコピー"
                  sx={{ bgcolor: copied ? '#10b981' : UI.tint, '&:hover': { bgcolor: copied ? '#059669' : UI.ruleSoft } }}
                >
                  {copied
                    ? <CheckIcon sx={{ color: '#fff', fontSize: 18 }} />
                    : <ContentCopyIcon sx={{ color: UI.inkSoft, fontSize: 18 }} />
                  }
                </IconButton>
              </Tooltip>
            )}
          </Box>
          <Stack direction="row" spacing={1} sx={{ mb: 1.5, flexWrap: 'wrap' }}>
            <Chip size="small" label="追加" sx={{ bgcolor: '#dcfce7', color: '#166534' }} />
            <Chip size="small" label="削除" sx={{ bgcolor: '#fee2e2', color: '#991b1b' }} />
          </Stack>
          {reviewDiff?.usedFallback && (
            <Alert severity="warning" role="status" sx={{ mb: 1.5 }}>
              文章が長いため差分を簡易表示しています。変更箇所を正確に示していない場合があります。
            </Alert>
          )}
          {reviewDiff &&
            !reviewDiff.original.some(segment => segment.kind === 'removed') &&
            !reviewDiff.improved.some(segment => segment.kind === 'added') && (
              <Alert severity="info" role="status" sx={{ mb: 1.5 }}>
                改善前後に差分はありませんでした。
              </Alert>
            )}
          <Typography sx={{ fontSize: 12, fontWeight: 700, color: UI.inkFaint, mb: 0.5 }}>
            改善後（追加をハイライト）
          </Typography>
          {reviewDiff && (
            <HighlightedDiffText
              segments={reviewDiff.improved}
              ariaLabel="追加箇所を示した改善後テキスト"
            />
          )}
          <CharCountNote result={reviewResult} charLimit={requestedCharLimit} />
          <Divider sx={{ my: 2 }} />
          <Typography sx={{ fontWeight: 700, fontSize: 13, mb: 1, color: UI.inkFaint }}>
            元の文章（削除をハイライト）
          </Typography>
          {reviewDiff && (
            <HighlightedDiffText
              segments={reviewDiff.original}
              anchors={anchoredFeedback}
              ariaLabel="削除箇所と指摘箇所を示した元の文章"
            />
          )}
        </Paper>
      )}
    </Stack>
  )
}
