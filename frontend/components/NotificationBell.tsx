'use client'

import { useCallback, useEffect, useState, type MouseEvent } from 'react'
import { usePathname, useRouter } from 'next/navigation'
import {
  Alert,
  Badge,
  Box,
  Button,
  CircularProgress,
  IconButton,
  List,
  ListItemButton,
  ListItemText,
  Popover,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import { visuallyHidden } from '@mui/utils'
import NotificationsNoneIcon from '@mui/icons-material/NotificationsNone'
import { authService } from '@/lib/auth'
import { shouldShowStudentBottomNav } from '@/lib/sidebar-nav'
import { useInterviewSessionActive } from '@/app/interview/sessionChrome'
import {
  NOTIFICATION_KIND_LABELS,
  fetchNotifications,
  fetchUnreadCount,
  markAllNotificationsRead,
  markNotificationRead,
  safeNotificationLink,
  type AppNotification,
} from '@/lib/notifications'

/** 未読の件数を見直す間隔。ページを開いたまま新しい通知に気づけるようにする。 */
const POLL_INTERVAL_MS = 60_000

function formatTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString('ja-JP', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

/**
 * ヘッダのベルと未読バッジ（#2033）。締切・選考結果・スカウト・先生の案内を見落とさないようにする。
 * 学生の画面（下のナビと同じ範囲）で、ログイン中だけ右上に出す。AI 面接の本番中は操作の邪魔になるので隠す。
 */
export function NotificationBell() {
  const pathname = usePathname()
  const router = useRouter()
  const interviewSessionActive = useInterviewSessionActive()
  const [loggedIn, setLoggedIn] = useState(false)
  const [unread, setUnread] = useState(0)
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  const [items, setItems] = useState<AppNotification[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const visible = loggedIn && !interviewSessionActive && (pathname === '/' || shouldShowStudentBottomNav(pathname))

  useEffect(() => {
    const user = authService.getStoredUser()
    setLoggedIn(Boolean(user && !user.is_admin && authService.getUserFetchHeaders()['X-User-Token']))
  }, [pathname])

  const refreshCount = useCallback(async () => {
    try {
      setUnread(await fetchUnreadCount())
    } catch {
      // バッジは補助の表示なので、取れなくても画面は止めない（次の見直しで取り直す）
    }
  }, [])

  useEffect(() => {
    if (!visible) return
    void refreshCount()
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') void refreshCount()
    }, POLL_INTERVAL_MS)
    const onFocus = () => void refreshCount()
    window.addEventListener('focus', onFocus)
    return () => {
      clearInterval(timer)
      window.removeEventListener('focus', onFocus)
    }
  }, [visible, refreshCount])

  const loadList = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await fetchNotifications()
      setItems(data.notifications)
      setUnread(data.unread_count)
    } catch {
      setError('通知を読み込めませんでした')
    } finally {
      setLoading(false)
    }
  }, [])

  const open = (e: MouseEvent<HTMLElement>) => {
    setAnchor(e.currentTarget)
    void loadList()
  }

  const openItem = async (n: AppNotification) => {
    if (!n.read_at) {
      setItems((prev) => prev.map((x) => (x.id === n.id ? { ...x, read_at: new Date().toISOString() } : x)))
      setUnread((u) => Math.max(0, u - 1))
      markNotificationRead(n.id).catch(() => void refreshCount())
    }
    const link = safeNotificationLink(n.link)
    if (link) {
      setAnchor(null)
      router.push(link)
    }
  }

  const readAll = async () => {
    try {
      await markAllNotificationsRead()
      setItems((prev) => prev.map((x) => (x.read_at ? x : { ...x, read_at: new Date().toISOString() })))
      setUnread(0)
    } catch {
      setError('既読にできませんでした。もう一度お試しください')
    }
  }

  if (!visible) return null

  return (
    <>
      <Tooltip title="通知">
        <IconButton
          onClick={open}
          aria-label={unread > 0 ? `通知（未読 ${unread} 件）` : '通知'}
          aria-haspopup="dialog"
          sx={{
            position: 'fixed',
            top: 8,
            right: 8,
            zIndex: (theme) => theme.zIndex.appBar + 1,
            bgcolor: 'background.paper',
            boxShadow: 1,
            '&:hover': { bgcolor: 'background.paper' },
          }}
        >
          <Badge badgeContent={unread} color="error" max={99}>
            <NotificationsNoneIcon />
          </Badge>
        </IconButton>
      </Tooltip>
      <Popover
        open={Boolean(anchor)}
        anchorEl={anchor}
        onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        slotProps={{ paper: { sx: { width: { xs: 'calc(100vw - 32px)', sm: 360 }, maxHeight: 480 } } }}
      >
        <Stack direction="row" sx={{ px: 2, py: 1.5, alignItems: 'center', justifyContent: 'space-between' }}>
          <Typography variant="subtitle1" component="h2" fontWeight={600}>通知</Typography>
          <Button size="small" onClick={() => void readAll()} disabled={unread === 0}>すべて既読にする</Button>
        </Stack>
        {error && (
          <Alert
            severity="error"
            sx={{ mx: 2, mb: 1 }}
            action={<Button color="inherit" size="small" onClick={() => void loadList()}>再読み込み</Button>}
          >
            {error}
          </Alert>
        )}
        {loading && items.length === 0 && (
          <Box textAlign="center" py={3}><CircularProgress size={20} aria-label="通知を読み込み中" /></Box>
        )}
        {!loading && !error && items.length === 0 && (
          <Typography color="text.secondary" sx={{ px: 2, pb: 2 }}>
            新しい通知はありません。選考の結果やスカウト、先生からの案内が届くとここに表示されます。
          </Typography>
        )}
        {items.length > 0 && (
          <List dense disablePadding sx={{ overflowY: 'auto' }}>
            {items.map((n) => (
              <ListItemButton
                key={n.id}
                onClick={() => void openItem(n)}
                sx={{ alignItems: 'flex-start', bgcolor: n.read_at ? undefined : 'action.hover' }}
              >
                <ListItemText
                  primary={
                    <Stack direction="row" spacing={1} sx={{ alignItems: 'baseline' }}>
                      <Typography variant="caption" color="primary">{NOTIFICATION_KIND_LABELS[n.kind] ?? 'お知らせ'}</Typography>
                      <Typography variant="body2" fontWeight={n.read_at ? 400 : 700}>
                        {n.read_at ? null : <Box component="span" sx={visuallyHidden}>未読 </Box>}
                        {n.title}
                      </Typography>
                    </Stack>
                  }
                  secondary={
                    <>
                      {n.body && <Typography component="span" variant="body2" display="block" color="text.secondary">{n.body}</Typography>}
                      <Typography component="span" variant="caption" color="text.secondary">{formatTime(n.created_at)}</Typography>
                    </>
                  }
                />
              </ListItemButton>
            ))}
          </List>
        )}
      </Popover>
    </>
  )
}
