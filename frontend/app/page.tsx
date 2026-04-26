'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Box, IconButton, AppBar, Toolbar, Typography } from '@mui/material'
import { Menu as MenuIcon } from '@mui/icons-material'
import { AnalysisSidebar } from '@/components/analysis-sidebar'
import { MuiChat } from '@/components/mui-chat'
import { authService, User } from '@/lib/auth'

export default function Home() {
  const router = useRouter()
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [mobileOpen, setMobileOpen] = useState(false)

  useEffect(() => {
    const storedUser = authService.getStoredUser()
    if (!storedUser) {
      router.replace('/login')
      return
    }
    setUser(storedUser)
    setLoading(false)
  }, [router])

  const handleLogout = () => {
    authService.logout()
    setUser(null)
    router.push('/login')
  }

  if (loading || !user) {
    return null
  }

  return (
    <Box sx={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      <AnalysisSidebar
        user={user}
        onLogout={handleLogout}
        mobileOpen={mobileOpen}
        onMobileClose={() => setMobileOpen(false)}
      />
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          height: '100vh',
          display: 'flex',
          flexDirection: 'column',
          minWidth: 0,
        }}
      >
        {/* モバイル用ヘッダー（ハンバーガーメニュー） */}
        <AppBar
          position="static"
          elevation={0}
          sx={{
            display: { xs: 'flex', md: 'none' },
            backgroundColor: '#fff',
            borderBottom: '1px solid #e0e0e0',
          }}
        >
          <Toolbar variant="dense" sx={{ minHeight: 48 }}>
            <IconButton
              edge="start"
              onClick={() => setMobileOpen(true)}
              sx={{ mr: 1, color: 'text.primary' }}
              aria-label="メニューを開く"
            >
              <MenuIcon />
            </IconButton>
            <Typography variant="subtitle1" sx={{ fontWeight: 600, color: 'text.primary' }}>
              IT業界キャリアエージェント
            </Typography>
          </Toolbar>
        </AppBar>
        <Box sx={{ flexGrow: 1, overflow: 'hidden' }}>
          <MuiChat />
        </Box>
      </Box>
    </Box>
  )
}
