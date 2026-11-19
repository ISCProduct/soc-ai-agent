'use client'

import { Box, Button, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'

type ResumeStartBarProps = {
  ready: boolean
  /** 準備ができたときの「ファイル名 ・ 応募先 ・ 区分」 */
  summary: string
  /** 準備ができていないときに、次にすることを伝える */
  hint: string
  onStart: () => void
}

/**
 * 画面の下に固定する開始バー。PC は左に要約・右にボタン、スマホは縦に並べる。
 * スマホでは下部ナビ（56px）の上に置く。
 */
export function ResumeStartBar({ ready, summary, hint, onStart }: ResumeStartBarProps) {
  return (
    <Box
      sx={{
        position: 'sticky',
        bottom: { xs: 'calc(56px + env(safe-area-inset-bottom))', md: 0 },
        zIndex: 2,
        bgcolor: UI.field,
        borderTop: `1px solid ${UI.rule}`,
      }}
    >
      <Box
        sx={{
          maxWidth: 1200,
          mx: 'auto',
          px: { xs: 2, md: 0 },
          py: { xs: 1.5, md: 2.25 },
          display: 'flex',
          flexDirection: { xs: 'column', md: 'row' },
          alignItems: { xs: 'stretch', md: 'center' },
          justifyContent: 'space-between',
          gap: { xs: 1, md: 2 },
        }}
      >
        <Typography
          sx={{ fontSize: 15, lineHeight: '24px', fontWeight: ready ? 700 : 400, color: ready ? UI.ink : UI.inkSoft, overflowWrap: 'anywhere' }}
        >
          {ready ? summary : hint}
        </Typography>
        <Box sx={{ display: 'flex', flexDirection: { xs: 'column-reverse', md: 'row' }, alignItems: 'center', gap: 2 }}>
          {/* 計測した値ではないので、目安の数字は出さない（#2217。ResumeRunningCard と同じ） */}
          <Typography sx={{ fontSize: 14, color: UI.inkSoft, whiteSpace: 'nowrap' }}>しばらくかかります</Typography>
          <Button
            variant="contained"
            disabled={!ready}
            onClick={onStart}
            sx={{ minHeight: 48, px: 3.5, fontSize: 16, width: { xs: '100%', md: 'auto' }, whiteSpace: 'nowrap' }}
          >
            レビューを始める
          </Button>
        </Box>
      </Box>
    </Box>
  )
}
