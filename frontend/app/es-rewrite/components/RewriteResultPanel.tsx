import { Box, Divider, IconButton, Paper, Stack, Tooltip, Typography } from '@mui/material'
import AutoFixHighIcon from '@mui/icons-material/AutoFixHigh'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import CheckIcon from '@mui/icons-material/Check'
import { UI } from '@/lib/design/tokens'
import type { RewriteResult, StarBreakdown } from '../types'
import CharCountNote from './CharCountNote'

// マーカーは S / T / A / R の頭文字を使う。
// 📍🎯⚡📊 を当てていたが、絵文字と項目の対応に意味が無く、
// STAR という枠組みそのものを隠していた。頭文字なら対応が自明。
const STAR_LABELS: { key: keyof StarBreakdown; label: string; color: string; initial: string }[] = [
  { key: 'situation', label: 'Situation（状況）', color: '#3b82f6', initial: 'S' },
  { key: 'task',      label: 'Task（課題）',      color: '#8b5cf6', initial: 'T' },
  { key: 'action',    label: 'Action（施策）',    color: UI.mark,   initial: 'A' },
  { key: 'result',    label: 'Result（成果）',    color: '#10b981', initial: 'R' },
]

type Props = {
  rewriteResult: RewriteResult
  // 比較用に表示する元の文章（送信時の入力欄の値）
  originalText: string
  requestedCharLimit: number | null
  copied: boolean
  onCopy: (text: string) => void
}

// ESリライトの結果（書き直し後の文章・STAR法分解・元の文章）
export default function RewriteResultPanel({ rewriteResult, originalText, requestedCharLimit, copied, onCopy }: Props) {
  return (
    <Stack spacing={2}>
      {/* Rewritten text */}
      <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `2px solid ${UI.mark}`, bgcolor: `color-mix(in srgb, ${UI.mark} 2%, transparent)` }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <AutoFixHighIcon sx={{ color: UI.mark, fontSize: 20 }} />
            <Typography sx={{ fontWeight: 700, fontSize: 17, color: UI.mark }}>リライト後</Typography>
          </Box>
          <Tooltip title={copied ? 'コピーしました' : 'クリップボードにコピー'}>
            <IconButton
              size="small"
              onClick={() => onCopy(rewriteResult.rewritten_text)}
              aria-label="リライト結果をコピー"
              sx={{ bgcolor: copied ? '#10b981' : UI.tint, '&:hover': { bgcolor: copied ? '#059669' : UI.ruleSoft } }}
            >
              {copied
                ? <CheckIcon sx={{ color: '#fff', fontSize: 18 }} />
                : <ContentCopyIcon sx={{ color: UI.inkSoft, fontSize: 18 }} />
              }
            </IconButton>
          </Tooltip>
        </Box>
        <Typography sx={{ fontSize: 14, lineHeight: 1.9, color: UI.ink, whiteSpace: 'pre-wrap' }}>
          {rewriteResult.rewritten_text}
        </Typography>
        <CharCountNote result={rewriteResult} charLimit={requestedCharLimit} />
      </Paper>

      {/* STAR breakdown */}
      <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `1px solid ${UI.ruleSoft}`, bgcolor: UI.field }}>
        <Typography sx={{ fontWeight: 700, fontSize: 16, mb: 0.5 }}>STAR法 分解</Typography>
        {/* 学生には初見の用語なので、見出しの下で一度だけ説明する。 */}
        <Typography sx={{ fontSize: 13, color: UI.inkFaint, mb: 2 }}>
          「どんな状況で、何が課題で、何をして、どうなったか」の4つが書けているかを見ます。
        </Typography>
        <Stack spacing={2}>
          {STAR_LABELS.map(({ key, label, color, initial }, idx) => (
            <Box key={key}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.8 }}>
                <Box sx={{
                  width: 28, height: 28, borderRadius: 1.5, bgcolor: `${color}15`,
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 14, fontWeight: 700, color,
                }}>
                  {initial}
                </Box>
                <Typography sx={{ fontWeight: 700, fontSize: 13, color }}>{label}</Typography>
              </Box>
              <Typography variant="body2" sx={{ color: UI.inkSoft, lineHeight: 1.75, borderLeft: `3px solid ${color}40`, pl: 1.5 }}>
                {rewriteResult.star[key] || '—'}
              </Typography>
              {idx < STAR_LABELS.length - 1 && <Divider sx={{ mt: 1.5, borderColor: UI.tint }} />}
            </Box>
          ))}
        </Stack>
      </Paper>

      {/* Compare hint */}
      <Paper elevation={0} sx={{ p: 2, borderRadius: 2, border: `1px solid ${UI.ruleSoft}`, bgcolor: UI.field }}>
        <Typography sx={{ fontWeight: 700, fontSize: 13, mb: 1, color: UI.inkFaint }}>元の文章</Typography>
        <Typography variant="body2" sx={{ color: UI.inkFaint, lineHeight: 1.8, whiteSpace: 'pre-wrap' }}>
          {originalText}
        </Typography>
      </Paper>
    </Stack>
  )
}
