'use client'

import { Box, Button, LinearProgress, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'

type ResumeReviewStatusProps = {
  loading: boolean
  hasReport: boolean
  error: string
  interrupted: boolean
  completed: boolean
  canRetry: boolean
  onRetry: () => void
}

function panelSx(accent: string) {
  return {
    p: 2,
    bgcolor: UI.tint,
    border: `1px solid ${UI.rule}`,
    borderLeft: `4px solid ${accent}`,
    borderRadius: 1,
  }
}

/**
 * レビューの進行・中断・失敗・完了を、ボタンのすぐ下に1か所で出す。
 * 色だけで伝えず、記号（◐ ⚠ ✕ ✓）と文を必ず添える。
 */
export function ResumeReviewStatus({
  loading,
  hasReport,
  error,
  interrupted,
  completed,
  canRetry,
  onRetry,
}: ResumeReviewStatusProps) {
  if (loading) {
    return (
      <Box role="status" sx={panelSx(UI.mark)}>
        <Typography fontWeight={700}>◐ {hasReport ? 'レポートを書いています' : '書類を読み取っています'}</Typography>
        <LinearProgress sx={{ my: 1.5 }} aria-label="レビューの進み具合" />
        <Typography variant="body2" sx={{ color: UI.inkSoft }}>
          通常30〜60秒かかります。この画面を開いたままお待ちください。
        </Typography>
      </Box>
    )
  }

  if (interrupted) {
    return (
      <Box role="alert" sx={panelSx(UI.flag)}>
        <Typography fontWeight={700}>⚠ 中断されました。もう一度お試しください。</Typography>
        <Typography variant="body2" sx={{ color: UI.inkSoft, mt: 0.5, mb: 1.5 }}>
          下のレポートは途中までで、完成版ではありません。もう一度試すと、最初から作り直します。
        </Typography>
        <Button variant="contained" onClick={onRetry} disabled={!canRetry} sx={{ width: { xs: '100%', sm: 'auto' } }}>
          もう一度試す
        </Button>
      </Box>
    )
  }

  if (error) {
    return (
      <Box role="alert" sx={panelSx(UI.flag)}>
        <Typography fontWeight={700}>✕ レビューを作成できませんでした</Typography>
        <Typography variant="body2" sx={{ color: UI.inkSoft, mt: 0.5 }}>
          {error}
        </Typography>
      </Box>
    )
  }

  if (completed) {
    return (
      <Box role="status" sx={panelSx(UI.mark)}>
        <Typography fontWeight={700}>✓ レビューが完了しました。下の指摘事項をご確認ください。</Typography>
      </Box>
    )
  }

  return null
}
