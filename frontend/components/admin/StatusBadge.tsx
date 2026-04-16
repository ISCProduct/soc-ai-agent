import { Chip } from '@mui/material'

const STATUS_CONFIG: Record<string, { label: string; color: 'default' | 'primary' | 'success' | 'warning' | 'error' }> = {
  draft: { label: '下書き', color: 'warning' },
  published: { label: '公開', color: 'success' },
  rejected: { label: '却下', color: 'error' },
  created: { label: '作成済み', color: 'default' },
  started: { label: '進行中', color: 'primary' },
  finished: { label: '完了', color: 'success' },
  error: { label: 'エラー', color: 'error' },
}

export function StatusBadge({ status, fallbackLabel = '下書き' }: { status?: string; fallbackLabel?: string }) {
  const normalized = status ?? 'draft'
  const config = STATUS_CONFIG[normalized]
  if (config) {
    return <Chip label={config.label} color={config.color} size="small" />
  }
  return <Chip label={status || fallbackLabel} color="default" size="small" />
}

