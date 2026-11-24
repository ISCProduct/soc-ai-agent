'use client'

import type { ReactNode } from 'react'
import { Typography } from '@mui/material'
import type { SxProps, Theme } from '@mui/material/styles'
import { UI } from '@/lib/design/tokens'

/**
 * 押せないボタンの理由を、常に見える文として出す（#2221）。
 * 吹き出し（Tooltip）はマウスを乗せないと出ず、タッチ端末では読めないので、吹き出しは補助にとどめる。
 */
export function DisabledReason({ id, children, sx }: { id: string; children: ReactNode; sx?: SxProps<Theme> }) {
  return (
    <Typography id={id} variant="caption" component="p" sx={[{ color: UI.inkSoft, mt: 0.5 }, ...(Array.isArray(sx) ? sx : [sx])]}>
      {children}
    </Typography>
  )
}
