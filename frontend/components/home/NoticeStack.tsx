'use client'

import { useState, type ReactNode } from 'react'
import { Box, Button } from '@mui/material'

export type Notice = { key: string; node: ReactNode }

/**
 * ホームのお知らせ（#1576）。件数に関わらず占有量を一定にするため、既定は先頭1件だけ出し、
 * 残りは「ほかN件」で開く。開いたときも高さを抑えて中でスクロールさせる
 * （先生の案内は最大10件・1件1000文字まで返る: guidance_service.go）。
 */
export function NoticeStack({ notices }: { notices: Notice[] }) {
  const [expanded, setExpanded] = useState(false)
  if (notices.length === 0) return null
  const rest = notices.length - 1
  const shown = expanded ? notices : notices.slice(0, 1)

  return (
    <Box component="section" aria-label="お知らせ">
      <Box id="home-notices" sx={{ maxHeight: expanded ? 240 : undefined, overflowY: expanded ? 'auto' : undefined }}>
        {shown.map((n) => <Box key={n.key}>{n.node}</Box>)}
      </Box>
      {rest > 0 && (
        <Box sx={{ px: 2, py: 0.5, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
          <Button size="small" aria-expanded={expanded} aria-controls="home-notices" onClick={() => setExpanded((v) => !v)}>
            {expanded ? 'お知らせを1件だけ表示' : `ほか${rest}件のお知らせを表示`}
          </Button>
        </Box>
      )}
    </Box>
  )
}
