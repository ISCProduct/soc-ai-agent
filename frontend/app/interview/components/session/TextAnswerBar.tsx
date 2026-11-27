'use client'

import { useState } from 'react'
import { Box, Button, TextField } from '@mui/material'

import { CALL } from '../../callTokens'

interface Props {
  disabled: boolean
  onSubmit: (text: string) => void
}

/**
 * 音声が使えないときの文字回答欄（#1831）。
 * 主操作は「送信」。マイクは ControlBar 側に残し、ここはテキスト専用。
 */
export default function TextAnswerBar({ disabled, onSubmit }: Props) {
  const [text, setText] = useState('')
  const trimmed = text.trim()
  const canSend = !disabled && trimmed.length > 0

  return (
    <Box
      component="form"
      onSubmit={(e) => {
        e.preventDefault()
        if (!canSend) return
        onSubmit(trimmed)
        setText('')
      }}
      sx={{
        display: 'flex',
        gap: 1,
        alignItems: 'flex-end',
        px: 2,
        pb: 1.5,
        pt: 0,
        flexShrink: 0,
        bgcolor: CALL.stage,
      }}
    >
      <TextField
        fullWidth
        multiline
        minRows={2}
        maxRows={4}
        size="small"
        label="文字で回答"
        placeholder="面接官の質問への回答を入力"
        value={text}
        disabled={disabled}
        onChange={(e) => setText(e.target.value)}
        inputProps={{ maxLength: 2000, 'aria-label': '文字で回答' }}
        sx={{
          '& .MuiOutlinedInput-root': {
            bgcolor: CALL.panel,
            color: CALL.ink,
            '& fieldset': { borderColor: CALL.line },
          },
          '& .MuiInputLabel-root': { color: CALL.muted },
        }}
      />
      <Button
        type="submit"
        variant="contained"
        disabled={!canSend}
        sx={{ flexShrink: 0, minHeight: 44, minWidth: 88, bgcolor: CALL.accent, '&:hover': { bgcolor: CALL.accent } }}
      >
        送信
      </Button>
    </Box>
  )
}
