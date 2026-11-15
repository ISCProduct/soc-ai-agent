import type { SxProps, Theme } from '@mui/material'
import { UI } from '@/lib/design/tokens'

/** タップ領域の最小サイズ（px）。モバイルで押し間違えないための下限。 */
export const MIN_TAP_SIZE = 44

/** 画面全体の最大幅。Figma の 2 カラム（プレビュー + 操作パネル）が収まる幅。 */
export const LOBBY_MAX_WIDTH = 1100

/** 右パネル（操作列）の幅。主ボタンや同意文が長くなりすぎない幅に抑える。 */
export const PANEL_WIDTH = 380

/** カメラの 16:9 枠。映像が無いときも枠の大きさが変わらないようにする。 */
export const previewFrameSx: SxProps<Theme> = {
  position: 'relative',
  width: '100%',
  aspectRatio: '16 / 9',
  bgcolor: UI.nightStage,
  borderRadius: 3,
  overflow: 'hidden',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
}

/** 情報カード（今回の目標・企業情報）の共通の面。 */
export const cardSx: SxProps<Theme> = {
  p: 2,
  bgcolor: UI.tint,
  border: `1px solid ${UI.ruleSoft}`,
  borderRadius: 2,
}

/** 文字だけのリンク風ボタン。tap 領域だけ 44px 確保し、見た目は文字のまま。 */
export const textLinkSx: SxProps<Theme> = {
  minHeight: MIN_TAP_SIZE,
  px: 0.5,
  color: UI.mark,
  textDecoration: 'underline',
  textTransform: 'none',
  fontSize: 13,
}
