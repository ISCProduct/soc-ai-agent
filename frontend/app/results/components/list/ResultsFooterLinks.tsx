'use client'

import type { ReactNode } from 'react'
import { Box, Button, Tooltip } from '@mui/material'
import { Refresh } from '@mui/icons-material'
import { GUEST_REGISTER_CTA_LABEL, GUEST_REGISTER_PATH, getGuestApplicationsButtonProps } from '@/lib/guest-limits'
import { UI } from '@/lib/design/tokens'

interface Props {
  isGuestUser: boolean
  /** SP のみ一覧の下に出すメール送信ボタン */
  emailAction: ReactNode
  onReset: () => void
  onNavigate: (path: string) => void
}

export function ResultsFooterLinks({ isGuestUser, emailAction, onReset, onNavigate }: Props) {
  const guest = getGuestApplicationsButtonProps(isGuestUser)
  return (
    <Box sx={{ mt: 3, mb: 4, display: 'grid', gap: 1.5 }}>
      {emailAction}
      <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1 }}>
        <Tooltip title={guest.title} disableHoverListener={!guest.disabled}>
          <span>
            <Button disabled={guest.disabled} onClick={() => onNavigate('/applications')} sx={{ color: UI.mark, minHeight: 44 }}>
              選考管理を見る
            </Button>
          </span>
        </Tooltip>
        <Button startIcon={<Refresh />} onClick={onReset} sx={{ color: UI.inkSoft, minHeight: 44 }}>
          最初からやり直す
        </Button>
        {isGuestUser && (
          <Button variant="contained" color="secondary" onClick={() => onNavigate(GUEST_REGISTER_PATH)}>
            {GUEST_REGISTER_CTA_LABEL}
          </Button>
        )}
      </Box>
    </Box>
  )
}
