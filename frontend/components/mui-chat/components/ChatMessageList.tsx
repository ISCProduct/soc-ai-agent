'use client'

import React from 'react'
import {
  Alert,
  Avatar,
  Box,
  Button,
  Chip,
  CircularProgress,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import { ErrorOutline, Person, SmartToy, WarningAmber } from '@mui/icons-material'
import styles from '../../mui-chat.module.css'
import { TypingIndicator } from './TypingIndicator'
import {
  CHAT_BRAND,
  CHAT_BRAND_HOVER,
  extractChoices,
  JOB_QUICK_OPTIONS,
  stripChoiceLines,
} from '../utils'
import type { Message } from '../types'

type ChatMessageListProps = {
  messages: Message[]
  isLoading: boolean
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
  historyLoadError,
  historyRetrying,
  messagesEndRef,
  messagesAreaRef,
  onRetryHistoryLoad,
  onQuickSelect,
}: ChatMessageListProps) {
  const hasUserMessage = messages.some((m) => m.role === 'user')
  const showQuickSelect = !historyLoadError && !hasUserMessage && messages.length > 0

  return (
    <Box
      ref={messagesAreaRef}
      className={styles.messagesArea}
      role="log"
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
            sx={{ bgcolor: CHAT_BRAND, '&:hover': { bgcolor: CHAT_BRAND_HOVER } }}
          >
            {historyRetrying ? '再読み込み中...' : '再試行'}
          </Button>
        </Box>
      )}

      {messages.map((message) => {
        const isValidationError =
          message.role === 'assistant' &&
          (message.content.includes('書かれた内容にはお答えできません') ||
            message.content.includes('質問に回答してください') ||
            message.content.includes('質問と関係のない内容が3回続いた'))

        const isTerminationMessage =
          message.role === 'assistant' &&
          message.content.includes('チャットを終了させていただきます')

        const choices =
          message.role === 'assistant' ? extractChoices(message.content) : []
        const displayContent =
          choices.length > 0 ? stripChoiceLines(message.content) : message.content

        return (
          <Box
            key={message.id}
            sx={{
              display: 'flex',
              mb: { xs: 2, md: 2.5 },
              justifyContent: message.role === 'user' ? 'flex-end' : 'flex-start',
            }}
          >
            {message.role === 'assistant' && (
              <Avatar
                sx={{
                  bgcolor: isTerminationMessage
                    ? '#d32f2f'
                    : isValidationError
                      ? '#f57c00'
                      : CHAT_BRAND,
                  width: 32,
                  height: 32,
                  mr: 1.5,
                  flexShrink: 0,
                }}
              >
                <SmartToy sx={{ fontSize: 18 }} />
              </Avatar>
            )}
            <Paper
              elevation={0}
              className={styles.messageBubble}
              sx={{
                backgroundColor:
                  message.role === 'user'
                    ? CHAT_BRAND
                    : isTerminationMessage
                      ? '#ffebee'
                      : isValidationError
                        ? '#fff3e0'
                        : '#f5f5f5',
                color: message.role === 'user' ? '#fff' : '#1a1a1a',
                border: isTerminationMessage
                  ? '2px solid #d32f2f'
                  : isValidationError
                    ? '2px solid #f57c00'
                    : message.role === 'assistant'
                      ? '1px solid #ebebeb'
                      : 'none',
              }}
            >
              {(isTerminationMessage || isValidationError) && (
                <Stack direction="row" alignItems="center" spacing={0.5} sx={{ mb: 0.5 }}>
                  {isTerminationMessage ? (
                    <ErrorOutline sx={{ fontSize: 16, color: '#d32f2f' }} />
                  ) : (
                    <WarningAmber sx={{ fontSize: 16, color: '#f57c00' }} />
                  )}
                  <Typography
                    variant="caption"
                    sx={{ fontWeight: 700, color: isTerminationMessage ? '#d32f2f' : '#f57c00' }}
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
            {message.role === 'user' && (
              <Avatar
                sx={{
                  bgcolor: '#757575',
                  width: 32,
                  height: 32,
                  ml: 1.5,
                  flexShrink: 0,
                }}
              >
                <Person sx={{ fontSize: 18 }} />
              </Avatar>
            )}
          </Box>
        )
      })}

      {isLoading && (
        <Box sx={{ display: 'flex', mb: 2, justifyContent: 'flex-start' }} role="status" aria-label="エージェントが入力中です">
          <Avatar
            sx={{
              bgcolor: CHAT_BRAND,
              width: 32,
              height: 32,
              mr: 1.5,
              flexShrink: 0,
            }}
          >
            <SmartToy sx={{ fontSize: 18 }} />
          </Avatar>
          <Paper
            elevation={0}
            className={styles.messageBubble}
            sx={{ backgroundColor: '#f5f5f5', border: '1px solid #ebebeb' }}
          >
            <TypingIndicator />
          </Paper>
        </Box>
      )}

      {showQuickSelect && (
        <Box sx={{ mt: 1, mb: 2, px: 1 }}>
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ mb: 1.5, textAlign: 'center' }}
          >
            クイック選択（タップで送信）
          </Typography>
          <Stack
            direction="row"
            spacing={1}
            justifyContent="center"
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
                  borderColor: CHAT_BRAND,
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
