import { Box, Button } from '@mui/material'
import AutoFixHighIcon from '@mui/icons-material/AutoFixHigh'
import RateReviewIcon from '@mui/icons-material/RateReview'
import { CONTENT_MAX_WIDTH, UI } from '@/lib/design/tokens'
import type { EsMode } from '../types'

type Props = {
  mode: EsMode
  // 送信中は切り替えさせない
  disabled: boolean
  // 切り替え時に消す結果・エラーは呼び出し側が決める
  onSelectReview: () => void
  onSelectRewrite: () => void
}

// ES添削 / ESリライト の切り替えボタン
export default function ModeToggle({ mode, disabled, onSelectReview, onSelectRewrite }: Props) {
  return (
    <Box sx={{ maxWidth: CONTENT_MAX_WIDTH, mx: 'auto', px: { xs: 2, md: 4 }, pt: 3 }}>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mb: 3 }}>
        <Button
          startIcon={<RateReviewIcon />}
          onClick={onSelectReview}
          disabled={disabled}
          sx={{
            px: 2.5, py: 1, borderRadius: 2, textTransform: 'none', fontWeight: 700, fontSize: 14,
            bgcolor: mode === 'review' ? UI.mark : UI.tint,
            color: mode === 'review' ? '#fff' : UI.inkSoft,
            '&:hover': { bgcolor: mode === 'review' ? `color-mix(in srgb, ${UI.mark} 88%, transparent)` : UI.ruleSoft },
          }}
        >
          ES添削
        </Button>
        <Button
          startIcon={<AutoFixHighIcon />}
          onClick={onSelectRewrite}
          disabled={disabled}
          sx={{
            px: 2.5, py: 1, borderRadius: 2, textTransform: 'none', fontWeight: 700, fontSize: 14,
            bgcolor: mode === 'rewrite' ? UI.mark : UI.tint,
            color: mode === 'rewrite' ? '#fff' : UI.inkSoft,
            '&:hover': { bgcolor: mode === 'rewrite' ? `color-mix(in srgb, ${UI.mark} 88%, transparent)` : UI.ruleSoft },
          }}
        >
          ESリライト（STAR法）
        </Button>
      </Box>
    </Box>
  )
}
