'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Box, IconButton, AppBar, Toolbar, Typography, Alert } from '@mui/material'
import { Menu as MenuIcon } from '@mui/icons-material'
import { AnalysisSidebar } from '@/components/analysis-sidebar'
import { MuiChat } from '@/components/mui-chat'
import { PageLoading } from '@/components/common/PageLoading'
import { authService, User } from '@/lib/auth'
import { WhatsNewEntry, fetchWhatsNewEntries, hasUnreadWhatsNew, markWhatsNewAsSeen } from '@/lib/whats-new-data'
import styles from './page.module.css'

export default function Home() {
  const router = useRouter()
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [whatsNewEntries, setWhatsNewEntries] = useState<WhatsNewEntry[]>([])
  const [showWhatsNewBanner, setShowWhatsNewBanner] = useState(false)

  useEffect(() => {
    const storedUser = authService.getStoredUser()
    if (!storedUser) {
      router.replace('/login')
      return
    }
    setUser(storedUser)
    setLoading(false)
    fetchWhatsNewEntries()
      .then((entries) => {
        setWhatsNewEntries(entries)
        setShowWhatsNewBanner(hasUnreadWhatsNew(entries))
      })
      .catch(() => {
        // 更新情報の取得失敗はチャット画面の利用を妨げない
      })
  }, [router])

  const dismissWhatsNewBanner = () => {
    markWhatsNewAsSeen(whatsNewEntries)
    setShowWhatsNewBanner(false)
  }

  const handleLogout = () => {
    authService.logout()
    setUser(null)
    router.push('/login')
  }

  if (loading || !user) {
    return <PageLoading message="チャット画面を準備しています..." />
  }

  return (
    <Box sx={{ display: 'flex', height: '100dvh', maxHeight: '100dvh', overflow: 'hidden' }}>
      <AnalysisSidebar
        user={user}
        onLogout={handleLogout}
        mobileOpen={mobileOpen}
        onMobileClose={() => setMobileOpen(false)}
      />
      <main className={styles.mainContent}>
        {/* モバイル用ヘッダー（ハンバーガーメニュー） */}
        <AppBar
          position="static"
          elevation={0}
          className={styles.mobileHeader}
          sx={{ backgroundColor: '#fff', borderBottom: '1px solid #e0e0e0' }}
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
        {showWhatsNewBanner && whatsNewEntries.length > 0 && (
          <Alert
            severity="info"
            onClose={dismissWhatsNewBanner}
            sx={{ borderRadius: 0 }}
            action={
              <Box
                component="a"
                href="/whats-new"
                sx={{ fontSize: '0.8rem', fontWeight: 600, color: 'inherit', textDecoration: 'underline', mr: 1, alignSelf: 'center' }}
              >
                詳しく見る
              </Box>
            }
          >
            新着情報: {whatsNewEntries[0].title}
          </Alert>
        )}
        <div className={styles.chatWrapper}>
          <MuiChat />
        </div>
      </main>
    </Box>
  )
}
