'use client'

import { Box, Button, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'

type ResumeReviewNoticeProps = {
  error: string
  interrupted: boolean
  canRetry: boolean
  onRetry: () => void
}

/**
 * 入力画面の上に出す、レビューがうまくいかなかった知らせ。
 * 色だけで伝えず、記号（⚠ ✕）と文を必ず添える。
 */
export function ResumeReviewNotice({ error, interrupted, canRetry, onRetry }: ResumeReviewNoticeProps) {
  if (!interrupted && !error) return null
  return (
    <Box
      role="alert"
      sx={{ p: 2, mb: 2.5, bgcolor: UI.flagTint, border: `1px solid ${UI.rule}`, borderLeft: `4px solid ${UI.flag}`, borderRadius: '8px' }}
    >
      {interrupted ? (
        <>
          <Typography sx={{ fontWeight: 700, color: UI.ink }}>⚠ 中断されました。もう一度お試しください。</Typography>
          <Typography variant="body2" sx={{ color: UI.inkSoft, mt: 0.5, mb: 1.5 }}>
            結果は途中までで、完成していません。もう一度試すと、最初から作り直します。
          </Typography>
          <Button variant="contained" onClick={onRetry} disabled={!canRetry} sx={{ width: { xs: '100%', sm: 'auto' } }}>
            もう一度試す
          </Button>
        </>
      ) : (
        <>
          <Typography sx={{ fontWeight: 700, color: UI.ink }}>✕ レビューを作成できませんでした</Typography>
          <Typography variant="body2" sx={{ color: UI.inkSoft, mt: 0.5 }}>
            {error}
          </Typography>
        </>
      )}
    </Box>
  )
}
