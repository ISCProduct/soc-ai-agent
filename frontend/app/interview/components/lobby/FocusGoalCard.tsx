import { Box, Typography } from '@mui/material'
import TrackChangesIcon from '@mui/icons-material/TrackChanges'
import { UI } from '@/lib/design/tokens'
import { cardSx } from './lobbyStyles'

export interface FocusGoal {
  text: string
}

/**
 * 「今回の目標」カード。前回レポートの課題を見せる想定。
 * 課題を取得する経路が未実装なので、値が無いときは何も描画しない（値を作らない）。
 */
export default function FocusGoalCard({ goal }: { goal?: FocusGoal | null }) {
  if (!goal?.text) return null
  return (
    <Box sx={cardSx}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, color: UI.mark, mb: 0.5 }}>
        <TrackChangesIcon sx={{ fontSize: 16 }} />
        <Typography sx={{ fontSize: 12, fontWeight: 700 }}>今回の目標</Typography>
      </Box>
      <Typography sx={{ fontSize: 16, fontWeight: 700, color: UI.ink }}>{goal.text}</Typography>
      <Typography sx={{ fontSize: 12, color: UI.inkSoft, mt: 0.5 }}>前回のレポートの課題です</Typography>
    </Box>
  )
}
