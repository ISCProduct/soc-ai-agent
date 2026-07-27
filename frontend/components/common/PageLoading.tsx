'use client'

import { Box, CircularProgress, Typography } from '@mui/material'

type PageLoadingProps = {
  message?: string
  showBrand?: boolean
  fullScreen?: boolean
}

export function PageLoading({
  message = '読み込み中...',
  showBrand = true,
  fullScreen = true,
}: PageLoadingProps) {
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 2,
        ...(fullScreen
          ? { minHeight: '100vh', bgcolor: '#f5f5f5' }
          : { flex: 1, minHeight: 240, width: '100%' }),
      }}
    >
      {showBrand && (
        <Typography variant="h6" sx={{ fontWeight: 700, color: '#ec5b13' }}>
          IT業界キャリアエージェント
        </Typography>
      )}
      <CircularProgress sx={{ color: '#ec5b13' }} />
      <Typography variant="body2" color="text.secondary">
        {message}
      </Typography>
    </Box>
  )
}
