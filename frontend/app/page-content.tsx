'use client'

import { useState, useEffect, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import { Box, Button, IconButton, AppBar, Toolbar, Typography, Alert, Link as MuiLink } from '@mui/material'
import NextLink from 'next/link'
import { Menu as MenuIcon } from '@mui/icons-material'
import { AnalysisSidebar } from '@/components/AnalysisSidebar'
import { MuiChat } from '@/components/mui-chat'
import { PageLoading } from '@/components/common/PageLoading'
import { authService, User } from '@/lib/auth'
import { WhatsNewEntry, fetchWhatsNewEntries, hasUnreadWhatsNew, markWhatsNewAsSeen } from '@/lib/whats-new-data'
import { fetchResumeStatus, resumeReminderMessage } from '@/lib/resume-reminder'
import {
  canDismiss,
  dismissGuidance,
  fetchActiveGuidances,
  withDismissing,
  withoutDismissing,
  type StudentGuidance,
} from '@/lib/teacher-guidance'
import { NextActionsBar } from '@/components/home/NextActionsBar'
import { NoticeStack, type Notice } from '@/components/home/NoticeStack'
import { AIBudgetNotice } from '@/components/home/AIBudgetNotice'
import styles from './page.module.css'

export default function PageContent() {
  const router = useRouter()
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [whatsNewEntries, setWhatsNewEntries] = useState<WhatsNewEntry[]>([])
  const [showWhatsNewBanner, setShowWhatsNewBanner] = useState(false)
  const [resumeReminder, setResumeReminder] = useState<string | null>(null)
  const [guidances, setGuidances] = useState<StudentGuidance[]>([])
  const [guidanceError, setGuidanceError] = useState('')
  const [guidanceLoadFailed, setGuidanceLoadFailed] = useState(false)
  // 閉じる処理中のID。連打すると同じ案内へ何度もPOSTが飛ぶ(教員側は disabled で防いでいる)
  const [dismissingIds, setDismissingIds] = useState<Set<number>>(() => new Set())

  // 教員案内の取得失敗はホームを壊さないが、「案内なし」とは見せない（#1691）。
  // 失敗を黙ると、教員が送った指導が届いていないことに学生が気づけない。
  const loadGuidances = useCallback(() => {
    setGuidanceLoadFailed(false)
    fetchActiveGuidances()
      .then((rows) => setGuidances(rows))
      .catch(() => setGuidanceLoadFailed(true))
  }, [])

  useEffect(() => {
    const storedUser = authService.getStoredUser()
    if (!storedUser) {
      router.replace('/login')
      return
    }
    // 職員（教員・キャリア担当）にはチャットを見せず、教員指導画面(/admin)へ送る。
    // 直リンクで / に来た場合もここで退避する。
    if (storedUser.is_staff) {
      router.replace('/admin')
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
    fetchResumeStatus()
      .then((status) => setResumeReminder(resumeReminderMessage(status)))
      .catch(() => {
        // 履歴書状態の取得失敗はホーム画面を壊さない（バナーを出さない）
      })
    loadGuidances()
  }, [router, loadGuidances])

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

  // お知らせは重要度順に並べ、既定では先頭1件だけ出す（#1576）。
  // 種類ごとに積むと、4種が同時に出たときチャット入力欄が画面外へ押し出される。
  const notices: Notice[] = []
  if (guidanceLoadFailed) {
    notices.push({
      key: 'guidance-load-failed',
      node: (
        <Alert
          severity="error"
          sx={{ borderRadius: 0 }}
          action={
            <Button color="inherit" size="small" onClick={loadGuidances}>
              再読み込み
            </Button>
          }
        >
          先生からの案内を読み込めませんでした。通信状況を確認して再読み込みしてください
        </Alert>
      ),
    })
  }
  if (guidanceError) {
    notices.push({
      key: 'guidance-error',
      node: (
        <Alert severity="error" onClose={() => setGuidanceError('')} sx={{ borderRadius: 0 }}>
          {guidanceError}
        </Alert>
      ),
    })
  }
  for (const g of guidances) {
    notices.push({
      key: `guidance-${g.id}`,
      node: (
        <Alert
          severity="info"
          // 閉じる処理中は無効化する。連打すると同じ案内へ何度もPOSTが飛ぶ。
          closeText="閉じる"
          onClose={
            !canDismiss(dismissingIds, g.id)
              ? undefined
              : () => {
                  setGuidanceError('')
                  setDismissingIds((prev) => withDismissing(prev, g.id))
                  dismissGuidance(g.id)
                    .then(() => setGuidances((prev) => prev.filter((x) => x.id !== g.id)))
                    .catch(() =>
                      setGuidanceError('案内を閉じられませんでした。時間をおいて再度お試しください'),
                    )
                    .finally(() => setDismissingIds((prev) => withoutDismissing(prev, g.id)))
                }
          }
          sx={{ borderRadius: 0 }}
        >
          <Box component="span" sx={{ fontWeight: 700 }}>先生から: </Box>
          {g.message}
          {g.suggested_industries && g.suggested_industries.length > 0 && (
            <>（参考: {g.suggested_industries.join('、')}）</>
          )}
        </Alert>
      ),
    })
  }
  if (resumeReminder) {
    notices.push({
      key: 'resume',
      node: (
        // MUI の Alert は action を渡すと onClose の閉じるボタンを描画しないため
        // (Alert.js: `action == null && onClose`)、導線は本文中に置いて閉じるボタンを残す。
        <Alert severity="warning" onClose={() => setResumeReminder(null)} sx={{ borderRadius: 0 }}>
          {resumeReminder}{' '}
          <MuiLink
            component={NextLink}
            href="/resume"
            sx={{ fontSize: '0.8rem', fontWeight: 600, color: 'inherit' }}
          >
            履歴書を確認する
          </MuiLink>
        </Alert>
      ),
    })
  }
  if (showWhatsNewBanner && whatsNewEntries.length > 0) {
    notices.push({
      key: 'whats-new',
      node: (
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
      ),
    })
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
          // 表示条件は sx に一本化する。
          // CSS Modules の `@media (min-width:900px){ display:none }` は
          // MUI が AppBar へ当てる `display:flex` に負けており、PCでも
          // 高さ49pxのモバイルヘッダーが残ってチャット内のタイトルと重複していた。
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
            {/* モバイル専用の見出し（#1479）。PCでは AppBar ごと非表示になり、
                代わりに ChatHeader 側のタイトルが h1 として見える。 */}
            <Typography variant="subtitle1" component="h1" sx={{ fontWeight: 600, color: 'text.primary' }}>
              IT業界キャリアエージェント
            </Typography>
          </Toolbar>
        </AppBar>
        <NextActionsBar />
        <AIBudgetNotice />
        <NoticeStack notices={notices} />
        <div className={styles.chatWrapper}>
          <MuiChat />
        </div>
      </main>
    </Box>
  )
}
