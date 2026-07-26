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
import { Person, SmartToy } from '@mui/icons-material'
import styles from '../../mui-chat.module.css'
import { TypingIndicator } from './TypingIndicator'
import { JOB_QUICK_OPTIONS } from '../utils'
import type { Message } from '../types'

type ChatMessageListProps = {
  messages: Message[]
  isLoading: boolean
  historyLoadError: string | null
  historyRetrying: boolean
  messagesEndRef: React.RefObject<HTMLDivElement | null>
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
  onRetryHistoryLoad,
  onQuickSelect,
}: ChatMessageListProps) {
  return (
    <Box
      className={styles.messagesArea}
      sx={{
        flexGrow: 1,
        overflowY: 'auto',
        backgroundColor: '#fff',
      }}
    >
      {historyLoadError && (
        <Box sx={{ textAlign: 'center', mt: 8, px: 3 }}>
          <Alert severity="error" sx={{ mb: 2, textAlign: 'left' }}>
            {historyLoadError}
          </Alert>
          <Button
            variant="contained"
            onClick={onRetryHistoryLoad}
            disabled={historyRetrying}
            startIcon={historyRetrying ? <CircularProgress size={16} color="inherit" /> : undefined}
          >
            {historyRetrying ? '再読み込み中...' : '再試行'}
          </Button>
        </Box>
      )}

      {messages.length === 0 && !historyLoadError && (
        <Box sx={{ textAlign: 'center', mt: 8 }}>
          <SmartToy sx={{ fontSize: 64, color: '#9e9e9e', mb: 2 }} />
          <Typography variant="h6" color="text.secondary" gutterBottom>
            こんにちは！IT業界専門のキャリアエージェントです。
          </Typography>
          <Typography variant="body2" color="text.secondary">
            4万社余りのIT企業の中から、あなたに最適な企業を選定いたします。
            <br />
            まず、どのような職種を希望されますか？
          </Typography>
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

        return (
          <Box
            key={message.id}
            sx={{
              display: 'flex',
              mb: 3,
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
                      : '#1976d2',
                  width: 36,
                  height: 36,
                  mr: 2,
                }}
              >
                <SmartToy sx={{ fontSize: 20 }} />
              </Avatar>
            )}
            <Paper
              elevation={1}
              className={styles.messageBubble}
              sx={{
                backgroundColor:
                  message.role === 'user'
                    ? '#1976d2'
                    : isTerminationMessage
                      ? '#ffebee'
                      : isValidationError
                        ? '#fff3e0'
                        : '#f5f5f5',
                color: message.role === 'user' ? '#fff' : '#000',
                border: isTerminationMessage
                  ? '2px solid #d32f2f'
                  : isValidationError
                    ? '2px solid #f57c00'
                    : 'none',
              }}
            >
              <Typography variant="body1">{message.content}</Typography>
            </Paper>
            {message.role === 'user' && (
              <Avatar
                sx={{
                  bgcolor: '#757575',
                  width: 36,
                  height: 36,
                  ml: 2,
                }}
              >
                <Person sx={{ fontSize: 20 }} />
              </Avatar>
            )}
          </Box>
        )
      })}

      {/* ローディングインジケーター */}
      {isLoading && (
        <Box
          sx={{
            display: 'flex',
            mb: 3,
            justifyContent: 'flex-start',
          }}
        >
          <Avatar
            sx={{
              bgcolor: '#1976d2',
              width: 36,
              height: 36,
              mr: 2,
            }}
          >
            <SmartToy sx={{ fontSize: 20 }} />
          </Avatar>
          <Paper
            elevation={1}
            className={styles.messageBubble}
            sx={{
              backgroundColor: '#f5f5f5',
            }}
          >
            <TypingIndicator />
          </Paper>
        </Box>
      )}

      {messages.length === 0 && (
        <Box sx={{ mt: 4 }}>
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ mb: 2, textAlign: 'center' }}
          >
            クイック選択：
          </Typography>
          <Stack
            direction="row"
            spacing={1}
            justifyContent="center"
            flexWrap="wrap"
            gap={1}
          >
            {JOB_QUICK_OPTIONS.map((option) => (
              <Chip
                key={option}
                label={option}
                onClick={() => onQuickSelect(option)}
                sx={{ cursor: 'pointer' }}
              />
            ))}
          </Stack>
        </Box>
      )}

      <div ref={messagesEndRef} />
    </Box>
  )
}
