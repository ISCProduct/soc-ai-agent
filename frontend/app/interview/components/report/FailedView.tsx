import { Box, Stack, Typography } from '@mui/material'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { UI } from '@/lib/design/tokens'
import { cardSx } from './reportStyles'

/** 失敗カード。再試行そのものの失敗も、生成中に戻さずここで知らせる（#1476） */
export default function FailedView({ detail, retryError }: { detail: string; retryError?: string }) {
  return (
    <Box sx={{ ...cardSx, border: `2px solid ${UI.flag}`, bgcolor: UI.tint }} role="alert">
      <Stack spacing={1}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: UI.flag }}>
          <ErrorOutlineIcon fontSize="small" />
          <Typography component="h2" sx={{ fontWeight: 700, color: 'inherit' }}>レポートを作れませんでした</Typography>
        </Box>
        <Typography variant="body2" sx={{ color: UI.inkSoft }}>{detail}</Typography>
        <ErrorAlert error={retryError ?? ''} sx={{ mb: 0 }} />
      </Stack>
    </Box>
  )
}
