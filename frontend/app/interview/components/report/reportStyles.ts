import type { SxProps, Theme } from '@mui/material'
import { UI } from '@/lib/design/tokens'

/** タップ領域の最小サイズ（px） */
export const MIN_TAP_SIZE = 44
/** 進捗バーの高さ（px） */
const PROGRESS_BAR_HEIGHT = 8
/** 画面全体の中央カラム幅（px）。Figma の進捗カード幅 */
const COLUMN_MAX_WIDTH = 432
/** レポート本体を出すときは、カードより広い幅にする（px） */
export const REPORT_BODY_MAX_WIDTH = 720

export const pageSx: SxProps<Theme> = {
  minHeight: '100vh',
  bgcolor: UI.paper,
  color: UI.ink,
  overflowY: 'auto',
  px: 2,
  py: { xs: 4, md: 6 },
}

export const columnSx = (maxWidth: number = COLUMN_MAX_WIDTH): SxProps<Theme> => ({
  maxWidth,
  mx: 'auto',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'stretch',
  gap: 2.5,
})

export const cardSx: SxProps<Theme> = {
  bgcolor: UI.tint,
  borderRadius: 3,
  p: { xs: 2, sm: 2.5 },
}

export const progressBarSx: SxProps<Theme> = {
  height: PROGRESS_BAR_HEIGHT,
  borderRadius: PROGRESS_BAR_HEIGHT / 2,
  bgcolor: UI.ruleSoft,
  '& .MuiLinearProgress-bar': { bgcolor: UI.mark },
}

/** 警告・エラー帯。色だけに頼らず、アイコンと文言を必ず併記する */
export const noticeSx: SxProps<Theme> = {
  display: 'flex',
  alignItems: 'flex-start',
  gap: 1,
  p: 2,
  borderRadius: 2,
  bgcolor: UI.field,
  border: `1px solid ${UI.flag}`,
  color: UI.flag,
}

export const primaryButtonSx: SxProps<Theme> = {
  minHeight: MIN_TAP_SIZE,
  borderRadius: 999,
  px: 3,
  textTransform: 'none',
  fontWeight: 700,
  bgcolor: UI.mark,
  color: UI.field,
  '&:hover': { bgcolor: UI.mark, opacity: 0.9 },
  '&.Mui-disabled': { bgcolor: UI.tint, color: UI.inkFaint },
}

export const secondaryButtonSx: SxProps<Theme> = {
  minHeight: MIN_TAP_SIZE,
  borderRadius: 999,
  px: 3,
  textTransform: 'none',
  fontWeight: 700,
  color: UI.ink,
  borderColor: UI.rule,
  bgcolor: UI.field,
  '&:hover': { borderColor: UI.ink, bgcolor: UI.field },
}

export const textLinkSx: SxProps<Theme> = {
  minHeight: MIN_TAP_SIZE,
  display: 'inline-flex',
  alignItems: 'center',
  alignSelf: 'center',
  color: UI.mark,
  textDecoration: 'underline',
  fontSize: 14,
}
