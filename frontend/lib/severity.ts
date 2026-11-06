/**
 * 指摘の重要度（critical / warning / info）の表示設定。
 * 履歴書レビューの AI 指摘と、ES・履歴書の形式チェック（#1530）で共通に使う。
 */

export type SeverityChipColor = 'error' | 'warning' | 'info' | 'default'

export type SeverityConfig = {
  color: SeverityChipColor
  label: string
  borderColor: string
}

/** 指摘事項の重要度表示設定 */
export const severityConfig: Record<string, Omit<SeverityConfig, 'color'> & { color: 'error' | 'warning' | 'info' }> = {
  critical: { color: 'error', label: '重大', borderColor: '#d32f2f' },
  warning: { color: 'warning', label: '注意', borderColor: '#ed6c02' },
  info: { color: 'info', label: '情報', borderColor: '#0288d1' },
}

/** 未知の severity にはデフォルト表示を返す */
export function getSeverityConfig(severity: string): SeverityConfig {
  return severityConfig[severity] ?? { color: 'default', label: severity, borderColor: '#9e9e9e' }
}
