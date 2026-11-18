'use client'

import { useId, useState, type FormEvent } from 'react'
import { Box, ButtonBase, InputBase, Typography } from '@mui/material'
import { HOME_COLORS } from '@/lib/home-theme'
import {
  CONSULT_FREE_INPUT_LABEL,
  CONSULT_FREE_INPUT_OPTIONAL,
  CONSULT_LABEL,
  CONSULT_NOTE,
  CONSULT_OPTIONS,
  CONSULT_QUESTION,
} from '@/lib/career-consult'

interface CareerConsultCardProps {
  /** 選んだ選択肢の文言、または入力した文章を回答として送る。 */
  onSend: (text: string) => void
  /** 送信中など。選択肢と送信を無効にする。 */
  disabled?: boolean
}

/**
 * ホームの「キャリア相談」カード（Figma「改善G／相談中心」ノード 37:407）。
 * 4つの選択肢（押すと、その文言を回答として送る）と、任意の自由入力。
 */
export function CareerConsultCard({ onSend, disabled = false }: CareerConsultCardProps) {
  const [text, setText] = useState('')
  const questionId = useId()
  const labelId = useId()
  const noteId = useId()
  const canSubmit = !disabled && text.trim().length > 0

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!canSubmit) return
    onSend(text.trim())
    setText('')
  }

  return (
    <Box
      component="section"
      aria-labelledby={questionId}
      aria-busy={disabled}
      sx={{
        bgcolor: HOME_COLORS.white,
        border: `1px solid ${HOME_COLORS.border}`,
        borderRadius: '8px',
        p: 3,
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
      }}
    >
      <Typography sx={{ fontWeight: 700, fontSize: 16, lineHeight: 1.7, color: HOME_COLORS.link }}>
        {CONSULT_LABEL}
      </Typography>
      <Typography
        id={questionId}
        component="h2"
        sx={{ fontWeight: 700, fontSize: 20, lineHeight: 1.5, color: HOME_COLORS.ink }}
      >
        {CONSULT_QUESTION}
      </Typography>

      <Box role="group" aria-labelledby={questionId} sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {CONSULT_OPTIONS.map((option) => (
          <ButtonBase
            key={option}
            disabled={disabled}
            onClick={() => onSend(option)}
            sx={{
              width: '100%',
              minHeight: 56,
              px: 2,
              justifyContent: 'flex-start',
              textAlign: 'left',
              bgcolor: HOME_COLORS.white,
              border: `1px solid ${HOME_COLORS.border}`,
              borderRadius: '8px',
              fontSize: 16,
              lineHeight: 1.7,
              color: HOME_COLORS.ink,
              fontFamily: 'inherit',
              '&:hover': { borderColor: HOME_COLORS.primary },
              '&.Mui-disabled': { opacity: 0.6 },
            }}
          >
            {option}
          </ButtonBase>
        ))}
      </Box>

      <Box component="form" onSubmit={submit} sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, fontSize: 16, lineHeight: 1.7 }}>
          <Typography id={labelId} component="span" sx={{ fontWeight: 700, color: HOME_COLORS.ink }}>
            {CONSULT_FREE_INPUT_LABEL}
          </Typography>
          <Typography component="span" sx={{ color: HOME_COLORS.soft }}>
            {CONSULT_FREE_INPUT_OPTIONAL}
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <InputBase
            value={text}
            onChange={(e) => setText(e.target.value)}
            disabled={disabled}
            inputProps={{ 'aria-labelledby': labelId, 'aria-describedby': noteId, maxLength: 1000 }}
            sx={{
              flex: 1,
              minWidth: 0,
              height: 48,
              px: 2,
              bgcolor: HOME_COLORS.white,
              border: `1px solid ${HOME_COLORS.inputBorder}`,
              borderRadius: '8px',
              fontSize: 16,
              color: HOME_COLORS.ink,
            }}
          />
          <ButtonBase
            type="submit"
            disabled={!canSubmit}
            sx={{
              minWidth: 96,
              height: 48,
              px: 3,
              bgcolor: HOME_COLORS.primary,
              color: HOME_COLORS.white,
              borderRadius: '8px',
              fontSize: 16,
              fontWeight: 700,
              fontFamily: 'inherit',
              '&.Mui-disabled': { opacity: 0.5, color: HOME_COLORS.white },
            }}
          >
            送る
          </ButtonBase>
        </Box>
      </Box>

      <Typography id={noteId} sx={{ fontSize: 16, lineHeight: 1.7, color: HOME_COLORS.soft }}>
        {CONSULT_NOTE}
      </Typography>
    </Box>
  )
}
