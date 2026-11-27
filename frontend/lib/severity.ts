/**
 * 指摘の重要度（critical / warning / info）の表示設定。
 * 履歴書レビューの AI 指摘と、ES・履歴書の形式チェック（#1530）で共通に使う。
 */

import type { SvgIconComponent } from '@mui/icons-material'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import HelpOutlineIcon from '@mui/icons-material/HelpOutline'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'

export type SeverityChipColor = 'error' | 'warning' | 'info' | 'default'

export type SeverityConfig = {
  color: SeverityChipColor
  label: string
  borderColor: string
  /**
   * 重要度の字形（#2218）。色だけでは、comfortable で error が黒になるときや高コントラストの設定で
   * 3種の差が色の濃さだけになるので、札は「色＋字形＋文字」で示す。3種は形の違う字形にする。
   */
  Icon: SvgIconComponent
}

/** 指摘事項の重要度表示設定 */
export const severityConfig: Record<string, Omit<SeverityConfig, 'color'> & { color: 'error' | 'warning' | 'info' }> = {
  critical: { color: 'error', label: '重大', borderColor: '#d32f2f', Icon: ErrorOutlineIcon },
  warning: { color: 'warning', label: '注意', borderColor: '#ed6c02', Icon: WarningAmberIcon },
  info: { color: 'info', label: '情報', borderColor: '#0288d1', Icon: InfoOutlinedIcon },
}

/** 未知の severity にはデフォルト表示を返す */
export function getSeverityConfig(severity: string): SeverityConfig {
  return severityConfig[severity] ?? { color: 'default', label: severity, borderColor: '#9e9e9e', Icon: HelpOutlineIcon }
}
