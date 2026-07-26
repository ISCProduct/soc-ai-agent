'use client'

import { Box, Button, Typography } from '@mui/material'
import styles from '../../mui-chat.module.css'
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
  const required = progressTotals?.required ?? totalQuestions
  const percent =
    progressTotals?.percent ?? Math.round((questionCount / totalQuestions) * 100)

  return (
    <Box className={styles.chatHeader}>
      <Box sx={{ minWidth: 0 }}>
        <Typography variant="h5" className={styles.chatTitle}>
          IT業界キャリアエージェント
        </Typography>
        <Typography variant="body2" color="text.secondary" className={styles.chatProgress}>
          AI適性診断 - {valid}/{required} 問完了
          {valid > 0 && ` (${percent}%)`}
        </Typography>
      </Box>
      <Button
        variant="outlined"
        size="small"
        onClick={onEndChat}
        className={styles.endButton}
      >
        終了
      </Button>
    </Box>
  )
}
