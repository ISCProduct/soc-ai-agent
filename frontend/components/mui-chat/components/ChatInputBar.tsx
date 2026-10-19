'use client'

import React from 'react'
import {
  Box,
  Button,
  Alert,
  IconButton,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import { Mic, MicOff, Send } from '@mui/icons-material'
import styles from '../MuiChat.module.css'
import type { ChoiceOption } from '../types'
import { CHAT_ACCENT, CHAT_ACCENT_HOVER, shouldSendChatOnKeyDown } from '../utils'
import { useSpeechInput } from '../hooks/useSpeechInput'

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
  // 音声で入れた内容は自動送信しない。音声認識は誤りが珍しくないので、
  // 送る前に直せるよう入力欄へ入れる。既に入力があれば後ろへ足す。
  const speech = useSpeechInput((text) => {
    onInputChange(input ? `${input}${input.endsWith('。') ? '' : ' '}${text}` : text)
    inputRef.current?.focus()
  })

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
            onClick={onShowCompletionModal}
            sx={{
              py: 2,
              px: 4,
              fontSize: '1.1rem',
              fontWeight: 'bold',
              bgcolor: CHAT_ACCENT,
              '&:hover': { bgcolor: CHAT_ACCENT_HOVER },
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
                              bgcolor: CHAT_ACCENT,
                              color: '#fff',
                              '&:hover': { bgcolor: CHAT_ACCENT_HOVER },
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
          {/*
            聞き取れなかったときは、何が起きたかと次にできることを出す。
            マイクが使えなくてもキーボードで入力できることを必ず添える。
          */}
          {speech.error && (
            <Alert severity="warning" sx={{ mb: 1 }} role="status">
              {speech.error}
            </Alert>
          )}
          <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-end' }}>
            {/*
              音声入力。使えない環境（Firefox など）ではボタン自体を出さない。
              押せるのに動かない状態より、無い方が分かりやすい。
              アイコンだけのボタンなので aria-label と Tooltip を付ける（§17）。
            */}
            {speech.supported && (
              <Tooltip
                title={
                  speech.listening
                    ? '音声入力を止める'
                    : speech.local
                      ? // 端末内で処理しているときだけそう書く。
                        // クラウドへ送っているのに「端末内」と書くと嘘になる。
                        '音声で入力する（この端末内で処理します）'
                      : '音声で入力する'
                }
              >
                <span>
                  <IconButton
                    onClick={() => (speech.listening ? speech.stop() : speech.start())}
                    disabled={isLoading || !!historyLoadError}
                    aria-label={speech.listening ? '音声入力を止める' : '音声で入力する'}
                    aria-pressed={speech.listening}
                    sx={{
                      width: 44,
                      height: 44,
                      mb: 0.5,
                      color: speech.listening ? CHAT_ACCENT : 'text.secondary',
                    }}
                  >
                    {speech.listening ? <Mic /> : <MicOff />}
                  </IconButton>
                </span>
              </Tooltip>
            )}
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
                bgcolor: CHAT_ACCENT,
                color: '#fff',
                mb: 2.5,
                '&:hover': {
                  bgcolor: CHAT_ACCENT_HOVER,
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
