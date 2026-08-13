'use client'

import { Box, Button, Stack, Typography } from '@mui/material'
import { useRouter } from 'next/navigation'

const PRIMARY = '#ec5b13'

type Props = {
  code?: string | number
  onRetry?: () => void
}

export function ServiceErrorView({ code, onRetry }: Props) {
  const router = useRouter()
  const label = code ? `エラー ${code}` : '接続エラー'

  return (
    <Box
      sx={{
        minHeight: '100dvh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: '#f8f6f6',
        px: 2,
      }}
    >
      <Stack spacing={2} alignItems="center" sx={{ maxWidth: 420, textAlign: 'center' }}>
        <Typography sx={{ fontSize: 13, fontWeight: 700, color: PRIMARY }}>{label}</Typography>
        <Typography variant="h5" fontWeight={700}>
          接続できません
        </Typography>
        <Typography color="text.secondary">
          サーバーに接続できませんでした。しばらくしてから再試行してください。
        </Typography>
        <Stack direction="row" spacing={1}>
          <Button variant="contained" onClick={() => (onRetry ? onRetry() : router.refresh())}>
            再試行
          </Button>
          <Button variant="outlined" onClick={() => router.push('/')}>
            トップへ
          </Button>
        </Stack>
      </Stack>
    </Box>
  )
}
