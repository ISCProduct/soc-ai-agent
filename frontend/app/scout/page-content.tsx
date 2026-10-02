'use client'

import Link from 'next/link'
import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Paper,
  Skeleton,
  Stack,
  Typography,
} from '@mui/material'
import { ArrowBack } from '@mui/icons-material'
import { authService } from '@/lib/auth'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import {
  SCOUT_STATUS_LABEL,
  studentScoutService,
  type StudentScout,
} from '@/lib/scout/api'

function formatWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ja-JP')
}

export default function PageContent() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [items, setItems] = useState<StudentScout[]>([])
  const [blockedIds, setBlockedIds] = useState<number[]>([])
  const [opened, setOpened] = useState<StudentScout | null>(null)
  const [blockTarget, setBlockTarget] = useState<StudentScout | null>(null)

  const load = useCallback(async () => {
    setError('')
    try {
      const data = await studentScoutService.list()
      setItems(data.items)
      setBlockedIds(data.blocked_company_ids || [])
    } catch (e) {
      setError(e instanceof Error ? e.message : 'スカウトを取得できませんでした')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user || user.is_guest) {
      router.replace('/login')
      return
    }
    void load()
  }, [router, load])

  const openScout = async (s: StudentScout) => {
    try {
      const viewed = await studentScoutService.view(s.id)
      setItems((prev) =>
        prev.map((x) => (x.id === s.id ? { ...x, status: viewed.status } : x)),
      )
      setOpened({ ...s, status: viewed.status, message: viewed.message || s.message })
    } catch (e) {
      setError(e instanceof Error ? e.message : '内容を開けませんでした')
    }
  }

  const decline = async (id: number) => {
    try {
      const res = await studentScoutService.decline(id)
      setItems((prev) => prev.map((x) => (x.id === id ? { ...x, status: res.status } : x)))
    } catch (e) {
      setError(e instanceof Error ? e.message : '辞退に失敗しました')
    }
  }

  const block = async (s: StudentScout) => {
    try {
      await studentScoutService.blockCompany(s.company_id)
      setBlockedIds((prev) => (prev.includes(s.company_id) ? prev : [...prev, s.company_id]))
      setBlockTarget(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'ブロックに失敗しました')
    }
  }

  if (loading) {
    return (
      <Box sx={{ maxWidth: 800, mx: 'auto', p: { xs: 2, sm: 3 } }}>
        <Skeleton variant="text" width={180} height={40} />
        <Skeleton variant="rectangular" height={120} sx={{ mt: 2 }} />
      </Box>
    )
  }

  return (
    <Box sx={{ maxWidth: 800, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Stack direction="row" alignItems="center" spacing={1} mb={1}>
        <IconButton onClick={() => router.back()} aria-label="戻る">
          <ArrowBack />
        </IconButton>
        <Typography variant="h5" component="h1" fontWeight="bold">
          受け取ったスカウト
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2, pl: 6 }}>
        企業からのオファーを確認できます。辞退やブロックもここから行えます。
      </Typography>

      {error && (
        <Alert
          severity="error"
          sx={{ mb: 2 }}
          action={
            <Button color="inherit" size="small" onClick={() => void load()}>
              再読み込み
            </Button>
          }
        >
          {error}
        </Alert>
      )}

      {blockedIds.length > 0 && (
        <Alert severity="info" sx={{ mb: 2 }}>
          ブロック中の企業があります（ID: {blockedIds.join('、')}）。新しいスカウトは届きません。
        </Alert>
      )}

      {items.length === 0 ? (
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
      ) : (
        <Stack spacing={1.5}>
          {items.map((s) => (
            <Paper key={s.id} variant="outlined" sx={{ p: 2 }}>
              <Stack
                direction="row"
                justifyContent="space-between"
                alignItems="flex-start"
                gap={1}
                flexWrap="wrap"
              >
                <Box>
                  <Typography fontWeight="bold">{s.company_name || `企業 #${s.company_id}`}</Typography>
                  <Typography variant="caption" color="text.secondary">
                    {formatWhen(s.created_at)}
                  </Typography>
                </Box>
                <Chip size="small" label={SCOUT_STATUS_LABEL[s.status]} />
              </Stack>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} sx={{ mt: 1.5 }}>
                <Button variant="contained" size="small" onClick={() => void openScout(s)}>
                  内容を確認
                </Button>
                {s.status !== 'declined' && (
                  <Button size="small" onClick={() => void decline(s.id)}>
                    このスカウトを辞退
                  </Button>
                )}
                <Button size="small" color="error" onClick={() => setBlockTarget(s)}>
                  この企業をブロック
                </Button>
              </Stack>
            </Paper>
          ))}
        </Stack>
      )}

      <Dialog open={Boolean(opened)} onClose={() => setOpened(null)} fullWidth maxWidth="sm">
        <DialogTitle>{opened?.company_name}からのスカウト</DialogTitle>
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
            {blockTarget?.company_name}
            からの新しいスカウトは届かなくなります。すでに届いている文面は履歴に残ります。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setBlockTarget(null)}>キャンセル</Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => blockTarget && void block(blockTarget)}
          >
            ブロックする
          </Button>
        </DialogActions>
      </Dialog>

      <BottomNavSpacer />
    </Box>
  )
}
