'use client'

import type { ReactNode } from 'react'
import NextLink from 'next/link'
import { Box, Button, Tooltip } from '@mui/material'
import { GUEST_REGISTER_CTA_LABEL, GUEST_REGISTER_PATH, getGuestApplicationsButtonProps } from '@/lib/guest-limits'
import { UI } from '@/lib/design/tokens'
import { DisabledReason } from './DisabledReason'

const APPLICATIONS_REASON_ID = 'results-applications-disabled-reason'

interface Props {
  isGuestUser: boolean
  /** SP のみ一覧の下に出すメール送信ボタン */
  emailAction: ReactNode
  onReset: () => void
}

const linkSx = {
  color: UI.ink,
  fontSize: 15,
  textDecoration: 'underline',
  textUnderlinePosition: 'from-font',
  minHeight: 44,
  minWidth: 44,
  px: 0,
  '&:hover': { textDecoration: 'underline', backgroundColor: 'transparent' },
  '&.Mui-disabled': { color: UI.inkFaint },
} as const

export function ResultsFooterLinks({ isGuestUser, emailAction, onReset }: Props) {
  const guest = getGuestApplicationsButtonProps(isGuestUser)
  return (
    <Box sx={{ mt: 2.5, mb: 4, display: 'grid', gap: 1.5 }}>
      {emailAction}
      <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', justifyContent: { xs: 'center', md: 'flex-start' }, columnGap: 3.5 }}>
        {/* 画面遷移はリンクにして、別タブでも開けるようにする。ゲストは押せないのでボタンのまま無効にする */}
        {guest.disabled ? (
          <Tooltip title={guest.title}>
            <span>
              <Button variant="text" disabled aria-describedby={APPLICATIONS_REASON_ID} sx={linkSx}>選考管理を見る</Button>
            </span>
          </Tooltip>
        ) : (
          <Button variant="text" component={NextLink} href="/applications" sx={linkSx}>選考管理を見る</Button>
        )}
        <Button variant="text" onClick={onReset} sx={linkSx}>最初からやり直す</Button>
        {isGuestUser && (
          <Button variant="contained" color="secondary" component={NextLink} href={GUEST_REGISTER_PATH}>
            {GUEST_REGISTER_CTA_LABEL}
          </Button>
        )}
      </Box>
      {/* 押せない理由は吹き出しだけでなく、常に見える文で出す（タッチ端末では吹き出しが出ない。#2221） */}
      {guest.disabled && (
        <DisabledReason id={APPLICATIONS_REASON_ID} sx={{ mt: 0, textAlign: { xs: 'center', md: 'left' } }}>{guest.title}</DisabledReason>
      )}
    </Box>
  )
}
