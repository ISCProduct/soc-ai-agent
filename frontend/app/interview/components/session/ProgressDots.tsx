import { Box } from '@mui/material'

import { CALL } from '../../callTokens'
import { progressDots, type ProgressDotState } from './sessionDerive'

const DOT_SX: Record<ProgressDotState, object> = {
  done: { width: 16, bgcolor: CALL.accent },
  current: { width: 20, bgcolor: CALL.ink },
  todo: { width: 16, bgcolor: CALL.line },
}

/** 質問ごとの進行セグメント。済=青、現在=白で長め、未=暗。装飾なので読み上げは親の文言に任せる。 */
export default function ProgressDots({ current, total }: { current: number; total: number }) {
  return (
    <Box aria-hidden sx={{ display: 'flex', gap: '3px' }}>
      {progressDots(current, total).map((state, i) => (
        <Box key={i} sx={{ height: 4, borderRadius: 2, ...DOT_SX[state] }} />
      ))}
    </Box>
  )
}
