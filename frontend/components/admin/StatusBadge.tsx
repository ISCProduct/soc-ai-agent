import { Chip } from '@mui/material'

type StatusColor = 'default' | 'primary' | 'success' | 'warning' | 'error'

/**
 * 状態ごとの表示定義。
 *
 * `mark` は色を読めない環境で区別をつけるための字形。
 * 以前は Chip の色だけで状態を分けていたため、色覚特性や白黒印刷、
 * 高コントラストモードでは「公開」と「却下」が判別できなかった（§19 違反）。
 */
const STATUS_CONFIG: Record<string, { mark: string; label: string; color: StatusColor }> = {
  draft: { mark: '○', label: '下書き', color: 'warning' },
  published: { mark: '●', label: '公開', color: 'success' },
  rejected: { mark: '×', label: '却下', color: 'error' },
  created: { mark: '○', label: '作成済み', color: 'default' },
  started: { mark: '◐', label: '進行中', color: 'primary' },
  finished: { mark: '✓', label: '完了', color: 'success' },
  error: { mark: '!', label: 'エラー', color: 'error' },
}

export function StatusBadge({ status, fallbackLabel = '下書き' }: { status?: string; fallbackLabel?: string }) {
  const normalized = status ?? 'draft'
  const config = STATUS_CONFIG[normalized]
  if (config) {
    // 字形はラベルの一部として渡す。aria-hidden にすると読み上げから消えるが、
    // Chip の label 全体が読み上げ対象なので記号も読まれてよい（「○ 下書き」）。
    return <Chip label={`${config.mark} ${config.label}`} color={config.color} size="small" />
  }
  // 未知の状態は色をつけない。知らない値に意味のある色を当てると誤読になる。
  return <Chip label={status || fallbackLabel} color="default" size="small" />
}
