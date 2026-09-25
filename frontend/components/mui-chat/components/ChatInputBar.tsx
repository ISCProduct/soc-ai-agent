'use client'

import React from 'react'
import {
  Box,
  Button,
  IconButton,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { Send } from '@mui/icons-material'
import styles from '../MuiChat.module.css'
import type { ChoiceOption } from '../types'
import { CHAT_BRAND, CHAT_BRAND_HOVER, shouldSendChatOnKeyDown } from '../utils'

type ChatInputBarProps = {
  analysisComplete: boolean
  showChoiceButtons: boolean
  choiceOptions: ChoiceOption[]
  selectedChoiceValue: string | null
  input: string
  inputPlaceholder: string
  isLoading: boolean
  historyLoadError: string | null
  canSend: boolean
  inputRef: React.RefObject<HTMLTextAreaElement | null>
  onInputChange: (value: string) => void
  onSelectChoice: (value: string) => void
  onSend: () => void
  onOtherChoice: () => void
  onShowCompletionModal: () => void
}

/** 選択肢チップ・入力欄・分析完了ボタン */
export function ChatInputBar({
  analysisComplete,
  showChoiceButtons,
  choiceOptions,
  selectedChoiceValue,
  input,
  inputPlaceholder,
  isLoading,
  historyLoadError,
  canSend,
  inputRef,
  onInputChange,
  onSelectChoice,
  onSend,
  onOtherChoice,
  onShowCompletionModal,
}: ChatInputBarProps) {
  return (
    <Box
      sx={{
        p: 2,
        borderTop: '1px solid #e0e0e0',
        backgroundColor: '#fff',
      }}
    >
      {analysisComplete ? (
        <Box sx={{ textAlign: 'center' }}>
          <Button
            variant="contained"
            size="large"
            onClick={() => {
              console.log('[MUI Chat] Rendering completion button (analysisComplete=true)')
              onShowCompletionModal()
            }}
            sx={{
              py: 2,
              px: 4,
              fontSize: '1.1rem',
              fontWeight: 'bold',
              bgcolor: CHAT_BRAND,
              '&:hover': { bgcolor: CHAT_BRAND_HOVER },
            }}
          >
            結果を見る
          </Button>
          <Typography variant="caption" display="block" sx={{ mt: 1 }} color="text.secondary">
            あなたに最適な企業をマッチングしました
          </Typography>
        </Box>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {showChoiceButtons && (
            <Paper
              elevation={0}
              sx={{
                p: 1.5,
                borderRadius: 2,
                border: '1px solid #e0e0e0',
                backgroundColor: '#fafafa',
              }}
            >
              <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
                選択肢を選んでください。任意で理由を入力してから送信できます（「その他」は自由記述）
              </Typography>
              <Stack direction="row" spacing={1} flexWrap="wrap" gap={1}>
                {choiceOptions.map((choice) => {
                  const isOtherChoice = choice.text.includes('その他')
                  const selected = selectedChoiceValue === choice.value
                  return (
                    <Button
                      key={`${choice.label}-${choice.text}`}
                      variant={selected ? 'contained' : 'outlined'}
                      onClick={() => {
                        if (isOtherChoice) {
                          onOtherChoice()
                          return
                        }
                        onSelectChoice(choice.value)
                      }}
                      disabled={isLoading}
                      className={styles.choiceButton}
                      sx={{
                        borderRadius: 2,
                        ...(selected
                          ? {
                              bgcolor: CHAT_BRAND,
                              color: '#fff',
                              '&:hover': { bgcolor: CHAT_BRAND_HOVER },
                            }
                          : {}),
                      }}
                    >
                      {choice.label}. {choice.text}
                    </Button>
                  )
                })}
              </Stack>
            </Paper>
          )}
          <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-end' }}>
            <TextField
              fullWidth
              multiline
              minRows={2}
              maxRows={6}
              placeholder={inputPlaceholder}
              value={input}
              onChange={(e) => {
                onInputChange(e.target.value)
              }}
              onKeyDown={(e) => {
                if (shouldSendChatOnKeyDown(e)) {
                  e.preventDefault()
                  if (canSend) onSend()
                }
              }}
              disabled={isLoading || !!historyLoadError}
              size="small"
              inputRef={inputRef}
              helperText="改行: Enter　／　送信: Ctrl+Enter（Mac は ⌘+Enter）"
              FormHelperTextProps={{ sx: { mx: 0.5 } }}
              sx={{
                '& .MuiOutlinedInput-root': {
                  borderRadius: 2,
                  alignItems: 'flex-start',
                },
              }}
            />
            <IconButton
              color="primary"
              onClick={() => onSend()}
              disabled={!canSend || isLoading || !!historyLoadError}
              aria-label="メッセージを送信"
              sx={{
                bgcolor: CHAT_BRAND,
                color: '#fff',
                mb: 2.5,
                '&:hover': {
                  bgcolor: CHAT_BRAND_HOVER,
                },
                '&.Mui-disabled': {
                  bgcolor: '#e0e0e0',
                },
              }}
            >
              <Send />
            </IconButton>
          </Box>
        </Box>
      )}
    </Box>
  )
}
