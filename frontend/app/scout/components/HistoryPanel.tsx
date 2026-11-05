'use client'

import { Chip, Paper, Stack, Typography } from '@mui/material'
import { SCOUT_STATUS_LABEL, type ScoutState } from '@/lib/scout/store'

function formatWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ja-JP')
}

export function HistoryPanel({ state }: { state: ScoutState }) {
  if (state.scouts.length === 0) {
    return (
      <Paper variant="outlined" sx={{ p: 3 }}>
        <Typography fontWeight="bold" gutterBottom>
          まだ送信したスカウトはありません
        </Typography>
        <Typography variant="body2" color="text.secondary">
          「送る」タブから学生を選んで送信すると、ここに履歴が残ります。
        </Typography>
      </Paper>
    )
  }

  return (
    <Stack spacing={1.5}>
      <Typography variant="body2" color="text.secondary">
        企業側・学生側の両方から同じ送信内容を確認できます。
      </Typography>
      {state.scouts.map((s) => (
        <Paper key={s.id} variant="outlined" sx={{ p: 2 }}>
          <Stack direction="row" justifyContent="space-between" alignItems="flex-start" gap={1} flexWrap="wrap">
            <Typography fontWeight="bold">{s.studentName}さん</Typography>
            <Chip size="small" label={SCOUT_STATUS_LABEL[s.status]} />
          </Stack>
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
            {formatWhen(s.createdAt)} ・ {s.templateTitle}
          </Typography>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
            {s.message}
          </Typography>
        </Paper>
      ))}
    </Stack>
  )
}
