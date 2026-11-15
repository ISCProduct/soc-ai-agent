'use client'

import { Button, Tooltip } from '@mui/material'
import { Email } from '@mui/icons-material'
import { getGuestEmailButtonProps } from '@/lib/guest-limits'
import { UI } from '@/lib/design/tokens'

interface Props {
  isGuestUser: boolean
  emailSending: boolean
  onSendEmail: () => void
  fullWidth?: boolean
}

export function EmailButton({ isGuestUser, emailSending, onSendEmail, fullWidth = false }: Props) {
  const guest = getGuestEmailButtonProps(isGuestUser)
  return (
    <Tooltip title={guest.title} disableHoverListener={!guest.disabled}>
      <span style={{ display: fullWidth ? 'block' : 'inline-block' }}>
        <Button
          variant="outlined"
          startIcon={<Email />}
          fullWidth={fullWidth}
          onClick={onSendEmail}
          disabled={emailSending || guest.disabled}
          sx={{ color: UI.ink, borderColor: UI.rule, backgroundColor: UI.field, minHeight: 44 }}
        >
          {emailSending ? '送信中...' : '結果をメールで受け取る'}
        </Button>
      </span>
    </Tooltip>
  )
}
