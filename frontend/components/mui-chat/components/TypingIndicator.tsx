'use client'

import { Box, CircularProgress, Typography } from '@mui/material'

/**
 * 返答待ちの表示。
 *
 * 「AIが考えています」をやめた。待たされている側に必要なのは
 * 「反応がある」ことであって、何が考えているかではない。
 */
export function TypingIndicator() {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <CircularProgress size={16} />
      <Typography variant="body2" color="text.secondary">
        考えています
      </Typography>
      <Box sx={{ display: 'flex', gap: 0.5 }}>
        {[0, 0.16, 0.32].map((delay, i) => (
          <Box
            key={i}
            sx={{
              width: 6,
              height: 6,
              borderRadius: '50%',
              bgcolor: 'text.secondary',
              animation: 'bounce 1.4s infinite ease-in-out',
              animationDelay: `${delay}s`,
              '@keyframes bounce': {
                '0%, 80%, 100%': { transform: 'scale(0)' },
                '40%': { transform: 'scale(1)' },
              },
            }}
          />
        ))}
      </Box>
    </Box>
  )
}
