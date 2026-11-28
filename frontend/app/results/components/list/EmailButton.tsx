'use client'

import { Box, Button, Tooltip } from '@mui/material'
import { Email } from '@mui/icons-material'
import { getGuestEmailButtonProps } from '@/lib/guest-limits'
import { UI } from '@/lib/design/tokens'
import { DisabledReason } from './DisabledReason'

// メール送信ボタンは PC・SP とも画面に1つだけ置くので、id は1つでよい
const REASON_ID = 'results-email-disabled-reason'

interface Props {
  isGuestUser: boolean
  emailSending: boolean
  onSendEmail: () => void
  fullWidth?: boolean
}

export function EmailButton({ isGuestUser, emailSending, onSendEmail, fullWidth = false }: Props) {
  const guest = getGuestEmailButtonProps(isGuestUser)
  return (
    <Box sx={{ display: fullWidth ? 'block' : 'inline-block' }}>
    <Tooltip title={guest.title} disableHoverListener={!guest.disabled}>
      <span style={{ display: fullWidth ? 'block' : 'inline-block' }}>
        <Button
          variant="outlined"
          startIcon={<Email />}
          fullWidth={fullWidth}
          onClick={onSendEmail}
          disabled={emailSending || guest.disabled}
          aria-describedby={guest.disabled ? REASON_ID : undefined}
          sx={{ color: UI.ink, borderColor: UI.rule, backgroundColor: UI.field, minHeight: 44 }}
        >
          {emailSending ? '送信中...' : '結果をメールで受け取る'}
        </Button>
      </span>
    </Tooltip>
    {guest.disabled && <DisabledReason id={REASON_ID}>{guest.title}</DisabledReason>}
    </Box>
  )
}
