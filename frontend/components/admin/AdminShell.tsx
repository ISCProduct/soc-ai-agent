'use client'

import { useEffect, useState, type ReactNode } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  Alert,
  AppBar,
  Box,
  Button,
  Divider,
  Drawer,
  IconButton,
  List,
  ListItemButton,
  ListItemText,
  Toolbar,
  Typography,
} from '@mui/material'
import MenuIcon from '@mui/icons-material/Menu'
import { authService } from '@/lib/auth'
import { adminTokenNeedsRefresh } from '@/lib/auth/admin-token'
import { getAdminSchoolAccess } from '@/lib/admin/school-access'
import { ADMIN_COLORS } from '@/lib/design-tokens'
import { activeAdminNavItem, visibleAdminNav } from '@/lib/admin/nav'

const SIDEBAR_WIDTH = 248
const MAIN_ID = 'admin-main'

/**
 * 管理画面の共通シェル。
 *
 * 27画面に共通レイアウトが無く、ハブ画面（/admin）から下位画面へ入ると
 * 「戻る」以外の移動手段が無かった。ここでサイドバーを常設して、
 * どの画面からでも他の管理機能へ移れるようにする（§15: 管理機能が多い場合はサイドバー）。
 *
 * 余白は持たせない。各ページが `PageContainer` で余白と最大幅を持っているため、
 * ここで付けると二重になる。
 */
export function AdminShell({ children }: { children: ReactNode }) {
  const pathname = usePathname() || ''
  const [isPlatform, setIsPlatform] = useState<boolean | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)
  // 期限が切れた管理者トークンのまま子画面を描くと、開いた瞬間に 403 になる。
  // 確認が終わるまで本文は出さない。
  const [adminTokenPhase, setAdminTokenPhase] = useState<'pending' | 'ready' | 'error'>('pending')
  const [adminTokenAttempt, setAdminTokenAttempt] = useState(0)

  useEffect(() => {
    let alive = true
    if (!adminTokenNeedsRefresh(authService.getStoredToken())) {
      setAdminTokenPhase('ready')
      return () => {
        alive = false
      }
    }
    // 表示中（ready）の遷移では本文を隠さない。隠すとアンマウントされ、遷移先の入力が消える。
    // 背景で取り直し、失敗してもまだ有効なトークンで動いている間は ready のまま残す。
    setAdminTokenPhase((phase) => (phase === 'ready' ? phase : 'pending'))
    authService.ensureFreshAdminToken()
      .then(() => {
        if (alive) setAdminTokenPhase('ready')
      })
      .catch(() => {
        if (alive) setAdminTokenPhase((phase) => (phase === 'ready' ? phase : 'error'))
      })
    return () => {
      alive = false
    }
  }, [pathname, adminTokenAttempt])

  useEffect(() => {
    if (adminTokenPhase !== 'ready') return
    let alive = true
    getAdminSchoolAccess()
      .then((access) => {
        if (alive) setIsPlatform(access.restricted === false)
      })
      .catch(() => {
        // 取得に失敗したらシステム管理を出さない方向に倒す（fail-closed）。
        // ハブ画面も同じ扱いにしてある。
        if (alive) setIsPlatform(false)
      })
    return () => {
      alive = false
    }
  }, [adminTokenPhase])

  // パス変更でモバイルのドロワーを閉じる。開いたまま遷移すると
  // 遷移先の内容が隠れたままになる。
  useEffect(() => {
    setDrawerOpen(false)
  }, [pathname])

  const groups = visibleAdminNav(isPlatform)
  const active = activeAdminNavItem(pathname)

  const nav = (
    <Box component="nav" aria-label="管理メニュー" sx={{ py: 1 }}>
      <List dense disablePadding>
        <ListItemButton
          component={Link}
          href="/admin"
          selected={pathname === '/admin'}
          aria-current={pathname === '/admin' ? 'page' : undefined}
          sx={{ minHeight: 44 }}
        >
          <ListItemText primary="管理メニュー" primaryTypographyProps={{ fontWeight: 700 }} />
        </ListItemButton>
      </List>

      {groups.map((group) => (
        <Box key={group.heading} sx={{ mt: 1.5 }}>
          <Typography
            variant="caption"
            component="h2"
            sx={{ px: 2, color: ADMIN_COLORS.muted, fontWeight: 700 }}
          >
            {group.heading}
          </Typography>
          <List dense disablePadding sx={{ mt: 0.5 }}>
            {group.items.map((item) => {
              const current = active?.href === item.href
              return (
                <ListItemButton
                  key={item.href}
                  component={Link}
                  href={item.href}
                  selected={current}
                  aria-current={current ? 'page' : undefined}
                  sx={{
                    minHeight: 44,
                    // 現在地は左の罫で示す。色だけに頼らない（§19）。
                    borderLeft: '3px solid',
                    borderLeftColor: current ? ADMIN_COLORS.indigo : 'transparent',
                  }}
                >
                  <ListItemText
                    primary={item.title}
                    primaryTypographyProps={{ fontWeight: current ? 700 : 400 }}
                  />
                </ListItemButton>
              )
            })}
          </List>
        </Box>
      ))}
    </Box>
  )

  return (
    <Box sx={{ display: 'flex', minHeight: '100dvh', bgcolor: ADMIN_COLORS.rail }}>
      {/* キーボード利用者がサイドバーを読み飛ばせるようにする */}
      <Box
        component="a"
        href={`#${MAIN_ID}`}
        sx={{
          position: 'absolute',
          left: -9999,
          top: 0,
          zIndex: 1400,
          p: 1.5,
          bgcolor: ADMIN_COLORS.paper,
          '&:focus': { left: 8, top: 8 },
        }}
      >
        本文へスキップ
      </Box>

      {/* デスクトップ: 常設サイドバー */}
      <Box
        component="aside"
        sx={{
          display: { xs: 'none', md: 'block' },
          width: SIDEBAR_WIDTH,
          flexShrink: 0,
          bgcolor: ADMIN_COLORS.paper,
          borderRight: '1px solid',
          borderRightColor: ADMIN_COLORS.rule,
        }}
      >
        {nav}
      </Box>

      {/* モバイル: 一時的なドロワー */}
      <Drawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        sx={{ display: { md: 'none' } }}
        PaperProps={{ sx: { width: SIDEBAR_WIDTH } }}
      >
        {nav}
      </Drawer>

      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <AppBar
          position="sticky"
          elevation={0}
          sx={{
            display: { md: 'none' },
            bgcolor: ADMIN_COLORS.paper,
            color: ADMIN_COLORS.ink,
            borderBottom: '1px solid',
            borderBottomColor: ADMIN_COLORS.rule,
          }}
        >
          <Toolbar sx={{ gap: 1 }}>
            <IconButton onClick={() => setDrawerOpen(true)} aria-label="管理メニューを開く" edge="start">
              <MenuIcon />
            </IconButton>
            <Typography variant="body1" fontWeight={700} noWrap>
              {active?.title ?? '管理メニュー'}
            </Typography>
          </Toolbar>
        </AppBar>

        <Box component="main" id={MAIN_ID} sx={{ flex: 1, minWidth: 0 }}>
          {adminTokenPhase === 'pending' && (
            <Typography role="status" sx={{ p: 3 }}>
              管理者セッションを確認しています
            </Typography>
          )}
          {adminTokenPhase === 'error' && (
            <Alert severity="error" sx={{ m: 3 }} action={
              <Button color="inherit" onClick={() => setAdminTokenAttempt((n) => n + 1)}>
                もう一度試す
              </Button>
            }>
              管理者セッションを更新できませんでした。このままでは管理画面の操作が拒否されます。通信を確認して、もう一度試してください。
            </Alert>
          )}
          {adminTokenPhase === 'ready' && children}
        </Box>
        <Divider />
      </Box>
    </Box>
  )
}
