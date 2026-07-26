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
import styles from '../../mui-chat.module.css'
import type { ChoiceOption } from '../types'

type ChatInputBarProps = {
  analysisComplete: boolean
  showChoiceButtons: boolean
  choiceOptions: ChoiceOption[]
  input: string
  inputPlaceholder: string
  isLoading: boolean
  historyLoadError: string | null
  inputRef: React.RefObject<HTMLInputElement | null>
  onInputChange: (value: string) => void
  onSend: (overrideMessage?: string) => void
  onOtherChoice: () => void
  onShowCompletionModal: () => void
}

/** 選択肢チップ・入力欄・分析完了ボタン */
export function ChatInputBar({
  analysisComplete,
  showChoiceButtons,
  choiceOptions,
  input,
  inputPlaceholder,
  isLoading,
  historyLoadError,
  inputRef,
  onInputChange,
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
            }}
          >
            🎉 分析完了！結果を見る
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
                選択肢を選んでください（同じ内容を入力しても構いません。「その他」は自由記述）
              </Typography>
              <Stack direction="row" spacing={1} flexWrap="wrap" gap={1}>
                {choiceOptions.map((choice) => {
                  const isOtherChoice = choice.text.includes('その他')
                  return (
                    <Button
                      key={`${choice.label}-${choice.text}`}
                      variant="outlined"
                      onClick={() => {
                        if (isOtherChoice) {
                          onOtherChoice()
                          return
                        }
                        onSend(choice.value)
                      }}
                      disabled={isLoading}
                      className={styles.choiceButton}
                      sx={{ borderRadius: 2 }}
                    >
                      {choice.label}. {choice.text}
                    </Button>
                  )
                })}
              </Stack>
            </Paper>
          )}
          <Box sx={{ display: 'flex', gap: 1 }}>
            <TextField
              fullWidth
              placeholder={inputPlaceholder}
              value={input}
              onChange={(e) => {
                onInputChange(e.target.value)
              }}
              onKeyPress={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  onSend()
                }
              }}
              disabled={isLoading || !!historyLoadError}
              size="small"
              inputRef={inputRef}
              sx={{
                '& .MuiOutlinedInput-root': {
                  borderRadius: 2,
                },
              }}
            />
            <IconButton
              color="primary"
              onClick={() => onSend()}
              disabled={!input.trim() || isLoading || !!historyLoadError}
              sx={{
                bgcolor: '#1976d2',
                color: '#fff',
                '&:hover': {
                  bgcolor: '#1565c0',
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
