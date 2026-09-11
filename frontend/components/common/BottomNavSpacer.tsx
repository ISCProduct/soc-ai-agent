import { Box } from '@mui/material'
import { BOTTOM_NAV_HEIGHT } from '@/lib/sidebar-nav'

/**
 * モバイルでは画面下部に StudentBottomNav が固定表示されるため、
 * スクロール末尾の CTA / 一覧がそれに隠れないよう余白を確保する。
 * デスクトップ（Bottom nav 非表示）では余白を持たない。
 */
export function BottomNavSpacer() {
  return <Box aria-hidden sx={{ height: { xs: BOTTOM_NAV_HEIGHT, md: 0 } }} />
}
