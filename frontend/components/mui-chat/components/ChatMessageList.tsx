'use client'

import React from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Skeleton,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import { ErrorOutline, WarningAmber } from '@mui/icons-material'
import styles from '../MuiChat.module.css'
import { TypingIndicator } from './TypingIndicator'
import {
  CHAT_ACCENT,
  CHAT_ACCENT_HOVER,
  CHAT_STOP_EDGE,
  CHAT_STOP_TEXT,
  CHAT_WARN_EDGE,
  CHAT_WARN_TEXT,
  extractChoices,
  isValidationFeedbackMessage,
  isValidationTerminationMessage,
  JOB_QUICK_OPTIONS,
  messageAccessibleLabel,
  stripChoiceLines,
} from '../utils'
import type { Message } from '../types'

type ChatMessageListProps = {
  messages: Message[]
  isLoading: boolean
  /** 履歴の初回読み込み中。true のあいだは吹き出しの骨格を出す */
  historyLoading?: boolean
  historyLoadError: string | null
  historyRetrying: boolean
  messagesEndRef: React.RefObject<HTMLDivElement | null>
  messagesAreaRef: React.RefObject<HTMLDivElement | null>
  onRetryHistoryLoad: () => void
  onQuickSelect: (option: string) => void
}

/** メッセージ一覧・履歴エラー・クイック選択・ローディング表示 */
export function ChatMessageList({
  messages,
  isLoading,
  historyLoading = false,
  historyLoadError,
  historyRetrying,
  messagesEndRef,
  messagesAreaRef,
  onRetryHistoryLoad,
  onQuickSelect,
}: ChatMessageListProps) {
  const hasUserMessage = messages.some((m) => m.role === 'user')
  const showQuickSelect = !historyLoadError && !hasUserMessage && messages.length > 0

  // ログには名前を与える。無いと読み上げでは「ログ」としか案内されず、
  // 何の一覧なのか分からない。
  return (
    <Box
      ref={messagesAreaRef}
      className={styles.messagesArea}
      role="log"
      aria-label="自己分析チャットのやり取り"
      aria-live="polite"
      aria-relevant="additions"
      sx={{
        flexGrow: 1,
        minHeight: 0,
        overflowY: 'auto',
        backgroundColor: '#fff',
      }}
    >
      {historyLoadError && (
        <Box sx={{ textAlign: 'center', mt: 4, px: 2 }}>
          <Alert severity="error" sx={{ mb: 2, textAlign: 'left' }}>
            {historyLoadError}
          </Alert>
          <Button
            variant="contained"
            onClick={onRetryHistoryLoad}
            disabled={historyRetrying}
            startIcon={historyRetrying ? <CircularProgress size={16} color="inherit" /> : undefined}
            sx={{ bgcolor: CHAT_ACCENT, '&:hover': { bgcolor: CHAT_ACCENT_HOVER } }}
          >
            {historyRetrying ? '再読み込み中...' : '再試行'}
          </Button>
        </Box>
      )}

      {/*
        履歴の読み込み中。履歴が0件でも挨拶が入るので、messages が空なのはこの間だけ。
        以前はここが空白で、通信が遅いと壊れているようにしか見えなかった（§23）。
        吹き出しの形に合わせた骨格にして、これから会話が出ることを示す。
      */}
      {historyLoading && messages.length === 0 && !historyLoadError && (
        <Box sx={{ px: { xs: 2, md: 3 }, pt: 2 }} aria-hidden>
          {[0, 1, 2].map((i) => (
            <Box
              key={i}
              sx={{
                display: 'flex',
                mb: { xs: 2, md: 2.5 },
                justifyContent: i % 2 === 1 ? 'flex-end' : 'flex-start',
              }}
            >
              <Skeleton
                variant="rounded"
                height={i % 2 === 1 ? 40 : 64}
                sx={{ width: i % 2 === 1 ? '45%' : '70%', borderRadius: 2 }}
              />
            </Box>
          ))}
        </Box>
      )}

      {messages.map((message) => {
        // 目印は utils の VALIDATION_FEEDBACK_MARKERS に寄せる。
        // ここだけ別の文字列を見ていると、文言を変えたときに
        // 片方の判定だけ外れて表示が崩れる。
        const isTerminationMessage =
          message.role === 'assistant' && isValidationTerminationMessage(message.content)

        const isValidationError =
          message.role === 'assistant' &&
          !isTerminationMessage &&
          isValidationFeedbackMessage(message.content)

        const choices =
          message.role === 'assistant' ? extractChoices(message.content) : []
        const displayContent =
          choices.length > 0 ? stripChoiceLines(message.content) : message.content

        return (
          <Box
            key={message.id}
            // 1件ずつを名前付きの区切りにする。発言者と時刻が無いと、
            // 読み上げでは誰の発言か分からないまま本文だけが並ぶ。
            component="article"
            aria-label={messageAccessibleLabel(message.role, message.timestamp)}
            sx={{
              display: 'flex',
              mb: { xs: 2, md: 2.5 },
              justifyContent: message.role === 'user' ? 'flex-end' : 'flex-start',
            }}
          >
            <Paper
              elevation={0}
              className={styles.messageBubble}
              sx={{
                // 聞き取りの記録として読ませる。
                //
                // 以前は左右に吹き出しを並べ、ロボットと人のアイコンを向かい合わせていた。
                // これは「AIとの雑談」の見た目で、15問の聞き取りという中身と合わない。
                //
                // 質問は枠で囲わず左の罫だけを引いて本文として読ませ、
                // 回答は右寄せで塗り、発言した側に向いた角を落として向きを示す。
                // 位置と形で区別するので、色が読めなくても誰の発言か分かる（§19）。
                backgroundColor:
                  message.role === 'user'
                    ? CHAT_ACCENT
                    : isTerminationMessage
                      ? '#FDF1EA'
                      : isValidationError
                        ? '#FDF6E7'
                        : 'transparent',
                color: message.role === 'user' ? '#fff' : '#1a1a1a',
                border: 'none',
                borderLeft:
                  message.role === 'assistant'
                    ? `3px solid ${
                        isTerminationMessage
                          ? CHAT_STOP_EDGE
                          : isValidationError
                            ? CHAT_WARN_EDGE
                            : CHAT_ACCENT
                      }`
                    : 'none',
                borderRadius:
                  message.role === 'user' ? '12px 12px 2px 12px' : '0 10px 10px 0',
                // 囲いが無いぶん行長が伸びすぎないよう上限を付ける。
                maxWidth: message.role === 'assistant' ? '46rem' : undefined,
              }}
            >
              {(isTerminationMessage || isValidationError) && (
                <Stack direction="row" alignItems="center" spacing={0.5} sx={{ mb: 0.5 }}>
                  {isTerminationMessage ? (
                    <ErrorOutline sx={{ fontSize: 16, color: CHAT_STOP_TEXT }} />
                  ) : (
                    <WarningAmber sx={{ fontSize: 16, color: CHAT_WARN_TEXT }} />
                  )}
                  <Typography
                    variant="caption"
                    sx={{ fontWeight: 700, color: isTerminationMessage ? CHAT_STOP_TEXT : CHAT_WARN_TEXT }}
                  >
                    {isTerminationMessage ? 'チャット終了' : '注意'}
                  </Typography>
                </Stack>
              )}
              <Typography
                variant="body1"
                sx={{ whiteSpace: 'pre-line', lineHeight: 1.65, fontSize: { xs: '0.9375rem', md: '1rem' } }}
              >
                {displayContent}
              </Typography>
            </Paper>
          </Box>
        )
      })}

      {isLoading && (
        <Box sx={{ display: 'flex', mb: 2, justifyContent: 'flex-start' }} role="status" aria-label="エージェントが入力中です">
          <Paper
            elevation={0}
            className={styles.messageBubble}
            sx={{ backgroundColor: 'transparent', border: 'none', borderLeft: `3px solid ${CHAT_ACCENT}`, borderRadius: '0 10px 10px 0' }}
          >
            <TypingIndicator />
          </Paper>
        </Box>
      )}

      {showQuickSelect && (
        <Box sx={{ mt: 1, mb: 2, px: 1 }}>
          {/*
            「クイック選択（タップで送信）」という見出しは外した。
            押せば送られることはボタンの見た目で分かるので、
            説明を足すとフォームではなくAIの機能紹介に見える。
            質問の直下に左から並べて、設問の選択肢として読ませる。
          */}
          <Stack
            direction="row"
            spacing={1}
            justifyContent="flex-start"
            flexWrap="wrap"
            useFlexGap
            gap={1}
          >
            {JOB_QUICK_OPTIONS.map((option) => (
              <Chip
                key={option}
                label={option}
                onClick={() => onQuickSelect(option)}
                clickable
                sx={{
                  cursor: 'pointer',
                  borderColor: CHAT_ACCENT,
                  '&:hover': { bgcolor: 'rgba(236,91,19,0.08)' },
                }}
                variant="outlined"
              />
            ))}
          </Stack>
        </Box>
      )}

      <div ref={messagesEndRef} />
    </Box>
  )
}
