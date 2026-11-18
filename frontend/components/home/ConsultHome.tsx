'use client'

import { Box, Typography } from '@mui/material'
import { HOME_COLORS, HOME_LAYOUT } from '@/lib/home-theme'
import { greetingFor } from '@/lib/home-greeting'
import type { ConsultIntent } from '@/lib/career-consult'
import { CareerConsultCard } from './CareerConsultCard'
import { ProgressStrip } from './ProgressStrip'

interface ConsultHomeProps {
  /** ログイン中の学生の名前（挨拶は姓だけを使う）。 */
  displayName: string
  onSend: (text: string, intent: ConsultIntent) => void
  disabled?: boolean
  progress: { valid: number; required: number; percent: number }
}

/**
 * 診断を始める前のホームのメイン（Figma「改善G／相談中心」ノード 37:403）。
 * 挨拶、「キャリア相談」カード、進捗の帯。診断が始まった学生は、従来のチャット表示に戻る。
 */
export function ConsultHome({ displayName, onSend, disabled = false, progress }: ConsultHomeProps) {
  return (
    <Box sx={{ height: '100%', overflowY: 'auto', bgcolor: HOME_COLORS.page }}>
      <Box
        sx={{
          maxWidth: HOME_LAYOUT.contentWidth + 80,
          mx: 'auto',
          px: { xs: 2, md: 5 },
          py: { xs: 3, md: 4 },
          display: 'flex',
          flexDirection: 'column',
          gap: 3,
        }}
      >
        <Box>
          <Typography
            component="h1"
            sx={{ fontWeight: 700, fontSize: { xs: 24, md: 32 }, lineHeight: 1.5, color: HOME_COLORS.ink }}
          >
            {greetingFor(displayName)}
          </Typography>
          <Typography sx={{ fontSize: 16, lineHeight: 1.7, color: HOME_COLORS.soft }}>
            進路について、気軽に相談できます。
          </Typography>
        </Box>
        <CareerConsultCard onSend={onSend} disabled={disabled} />
        <ProgressStrip percent={progress.percent} />
      </Box>
    </Box>
  )
}
