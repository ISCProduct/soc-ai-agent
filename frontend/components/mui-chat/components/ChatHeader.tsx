'use client'

import { Box, Button, LinearProgress, Typography } from '@mui/material'
import styles from '../../mui-chat.module.css'
import { CHAT_BRAND } from '../utils'
import type { ProgressTotals } from '../types'

type ChatHeaderProps = {
  progressTotals: ProgressTotals | null
  questionCount: number
  totalQuestions: number
  onEndChat: () => void
}

/** チャット上部のタイトル・進捗・終了ボタン */
export function ChatHeader({
  progressTotals,
  questionCount,
  totalQuestions,
  onEndChat,
}: ChatHeaderProps) {
  const valid = progressTotals?.valid ?? questionCount
  const required = progressTotals?.required ?? Math.max(1, totalQuestions)
  const percent =
    progressTotals?.percent ?? Math.min(100, Math.round((questionCount / required) * 100))

  return (
    <Box className={styles.chatHeader}>
      <Box sx={{ minWidth: 0, flex: 1, pr: 1 }}>
        <Typography variant="h5" className={styles.chatTitle}>
          IT業界キャリアエージェント
        </Typography>
        <Typography variant="body2" color="text.secondary" className={styles.chatProgress}>
          AI適性診断 — {valid}/{required} 問完了（想定{required}問・{percent}%）
        </Typography>
        <LinearProgress
          variant="determinate"
          value={percent}
          sx={{
            mt: 1,
            height: 6,
            borderRadius: 3,
            bgcolor: 'rgba(236,91,19,0.12)',
            '& .MuiLinearProgress-bar': { bgcolor: CHAT_BRAND },
          }}
        />
      </Box>
      <Button
        variant="outlined"
        size="small"
        onClick={onEndChat}
        className={styles.endButton}
        sx={{ borderColor: CHAT_BRAND, color: CHAT_BRAND, flexShrink: 0 }}
      >
        終了
      </Button>
    </Box>
  )
}
