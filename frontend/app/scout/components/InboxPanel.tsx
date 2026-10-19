'use client'

import Link from 'next/link'
import { useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import { SCOUT_STATUS_LABEL, type ScoutMessage, type ScoutState } from '@/lib/scout/store'

function formatWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ja-JP')
}

export function InboxPanel({
  state,
  onOpen,
  onDecline,
  onBlock,
}: {
  state: ScoutState
  onOpen: (id: number) => void
  onDecline: (id: number) => void
  onBlock: (companyName: string) => void
}) {
  const [opened, setOpened] = useState<ScoutMessage | null>(null)
  const [blockTarget, setBlockTarget] = useState<string | null>(null)

  if (state.scouts.length === 0) {
    return (
      <Paper variant="outlined" sx={{ p: 3 }}>
        <Typography fontWeight="bold" gutterBottom>
          届いているスカウトはまだありません
        </Typography>
        <Typography variant="body2" color="text.secondary">
          企業が送るとここに表示されます。公開設定はプロフィールから変更できます。
        </Typography>
        <Button component={Link} href="/profile" sx={{ mt: 2 }}>
          公開設定を確認
        </Button>
      </Paper>
    )
  }

  return (
    <Stack spacing={1.5}>
      <Typography variant="body2" color="text.secondary">
        内容を開くと既読になります。辞退と、今後その企業から受け取らない設定ができます。
      </Typography>
      {state.blockedCompanyIds.length > 0 && (
        <Alert severity="info">ブロック中: {state.blockedCompanyIds.join('、')}</Alert>
      )}
      {state.scouts.map((s) => (
        <Paper key={s.id} variant="outlined" sx={{ p: 2 }}>
          <Stack direction="row" justifyContent="space-between" alignItems="flex-start" gap={1} flexWrap="wrap">
            <Box>
              <Typography fontWeight="bold">{s.companyName}</Typography>
              <Typography variant="caption" color="text.secondary">
                {formatWhen(s.createdAt)}
              </Typography>
            </Box>
            <Chip size="small" label={SCOUT_STATUS_LABEL[s.status]} />
          </Stack>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} sx={{ mt: 1.5 }}>
            <Button
              variant="contained"
              size="small"
              onClick={() => {
                onOpen(s.id)
                setOpened(s)
              }}
            >
              内容を確認
            </Button>
            {s.status !== 'declined' && (
              <Button size="small" onClick={() => onDecline(s.id)}>
                このスカウトを辞退
              </Button>
            )}
            <Button size="small" color="error" onClick={() => setBlockTarget(s.companyName)}>
              この企業をブロック
            </Button>
          </Stack>
        </Paper>
      ))}

      <Dialog open={Boolean(opened)} onClose={() => setOpened(null)} fullWidth maxWidth="sm">
        <DialogTitle>{opened?.companyName}からのスカウト</DialogTitle>
        <DialogContent>
          <Typography variant="body1" sx={{ whiteSpace: 'pre-wrap', pt: 1 }}>
            {opened?.message}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOpened(null)}>閉じる</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={Boolean(blockTarget)} onClose={() => setBlockTarget(null)}>
        <DialogTitle>この企業をブロックしますか</DialogTitle>
        <DialogContent>
          <Typography>
            {blockTarget}からの新しいスカウトは届かなくなります。すでに届いている文面は履歴に残ります。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setBlockTarget(null)}>キャンセル</Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => {
              if (blockTarget) onBlock(blockTarget)
              setBlockTarget(null)
            }}
          >
            ブロックする
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  )
}
