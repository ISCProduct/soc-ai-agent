'use client'

import { Box, Typography } from '@mui/material'
import { UI, positionInBand } from '@/lib/design/tokens'

const BAR_WIDTH = 120
const DOT = 14

/** 候補全体の幅の中での位置を示す棒。装飾ではなく「候補の中での位置」。 */
export function FitBand({ score, band }: { score: number; band: { min: number; max: number } }) {
  const pos = positionInBand(score, band)
  return (
    <Box
      role="img"
      aria-label={`候補全体の幅の中での位置 ${Math.round(pos * 100)}%`}
      sx={{ position: 'relative', width: BAR_WIDTH, height: 6, borderRadius: 3, backgroundColor: UI.ruleSoft, flexShrink: 0 }}
    >
      <Box
        sx={{
          position: 'absolute',
          top: '50%',
          // 端でも丸が棒からはみ出さないよう、丸の幅ぶんを引いて配置する
          left: `calc(${pos * 100}% - ${pos * DOT}px)`,
          width: DOT,
          height: DOT,
          transform: 'translateY(-50%)',
          borderRadius: '50%',
          backgroundColor: UI.mark,
          border: `2px solid ${UI.field}`,
          boxSizing: 'border-box',
        }}
      />
    </Box>
  )
}

/** 「適合度」ラベル + 数値 + 位置バー。PC は縦、SP は横並び。 */
export function FitScore({ score, band, isDesktop }: { score: number; band: { min: number; max: number }; isDesktop: boolean }) {
  const label = (
    <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1 }}>
      <Typography component="span" sx={{ fontSize: 12, color: UI.inkSoft }}>適合度</Typography>
      <Typography component="span" sx={{ fontSize: 28, fontWeight: 700, color: UI.ink, lineHeight: 1, fontVariantNumeric: 'tabular-nums' }}>
        {score}
      </Typography>
    </Box>
  )
  return (
    <Box sx={{ display: 'flex', flexDirection: isDesktop ? 'column' : 'row', alignItems: isDesktop ? 'flex-end' : 'center', gap: isDesktop ? 0.75 : 1.5 }}>
      {label}
      <FitBand score={score} band={band} />
    </Box>
  )
}
