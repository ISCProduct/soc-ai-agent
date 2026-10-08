'use client'

import { Box, CircularProgress, Typography } from '@mui/material'

/**
 * 返答待ちの表示。
 *
 * 「AIが考えています」をやめた。待たされている側に必要なのは
 * 「反応がある」ことであって、何が考えているかではない。
 *
 * 動きを減らす設定（prefers-reduced-motion）では、跳ねる点と回転を止める。
 * 前庭障害や光過敏のある利用者には、繰り返し動く要素が体調不良の原因になる。
 * 止めても「考えています」の文字が残るので、待ち状態は伝わる。
 */
export function TypingIndicator() {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <CircularProgress
        size={16}
        // 動きを止める場合は回転させず、位置だけ示す輪にする。
        sx={{
          '@media (prefers-reduced-motion: reduce)': {
            animation: 'none',
            '& .MuiCircularProgress-circle': { animation: 'none' },
          },
        }}
      />
      <Typography variant="body2" color="text.secondary">
        考えています
      </Typography>
      <Box
        sx={{
          display: 'flex',
          gap: 0.5,
          // 跳ねる点は装飾なので、動きを減らす設定では出さない。
          '@media (prefers-reduced-motion: reduce)': { display: 'none' },
        }}
        aria-hidden
      >
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
