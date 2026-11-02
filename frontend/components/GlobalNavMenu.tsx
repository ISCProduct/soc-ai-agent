'use client'

import { useState } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import {
  Box,
  Drawer,
  Fab,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
} from '@mui/material'
import MenuIcon from '@mui/icons-material/Menu'
import BorderAllIcon from '@mui/icons-material/BorderAll'
import HistoryIcon from '@mui/icons-material/History'
import DescriptionIcon from '@mui/icons-material/Description'
import RecordVoiceOverIcon from '@mui/icons-material/RecordVoiceOver'
import AssessmentIcon from '@mui/icons-material/Assessment'
import EditNoteIcon from '@mui/icons-material/EditNote'
import CalendarMonthIcon from '@mui/icons-material/CalendarMonth'
import AssignmentIcon from '@mui/icons-material/Assignment'
import MailOutlineIcon from '@mui/icons-material/MailOutline'
import ManageAccountsIcon from '@mui/icons-material/ManageAccounts'
import HomeIcon from '@mui/icons-material/Home'
import FactCheckIcon from '@mui/icons-material/FactCheck'
import QuizIcon from '@mui/icons-material/Quiz'
import {
  GLOBAL_NAV_HOME_ITEMS,
  SIDEBAR_NAV_ITEMS,
  isCurrentNav,
  shouldShowStudentBottomNav,
} from '@/lib/sidebar-nav'
import { getResultsPathOrChat } from '@/lib/results-navigation'
import { useInterviewSessionActive } from '@/app/interview/sessionChrome'

type NavHref = (typeof SIDEBAR_NAV_ITEMS)[number]['href'] | (typeof GLOBAL_NAV_HOME_ITEMS)[number]['href']

const NAV_ICONS: Record<NavHref, React.ReactNode> = {
  '/': <HomeIcon color="primary" />,
  '/results': <FactCheckIcon color="primary" />,
  '/correlation-diagram': <BorderAllIcon color="primary" />,
  '/chat-history': <HistoryIcon color="primary" />,
  '/resume': <DescriptionIcon color="primary" />,
  '/assessments': <QuizIcon color="primary" />,
  '/ability-assessments': <QuizIcon color="primary" />,
  '/interview': <RecordVoiceOverIcon color="primary" />,
  '/interview/history': <AssessmentIcon color="primary" />,
  '/es-rewrite': <EditNoteIcon color="primary" />,
  '/schedule': <CalendarMonthIcon color="primary" />,
  '/applications': <AssignmentIcon color="primary" />,
  '/scout': <MailOutlineIcon color="primary" />,
  '/profile': <ManageAccountsIcon color="primary" />,
}

/**
 * ホーム画面以外のページから主要機能（ES・スケジュール・選考管理・相関図・面接履歴など）へ
 * 到達するための共通メニュー。ホームは AnalysisSidebar が同等の導線を持つため表示しない。
 * チャット進捗などホーム固有の責務は持たせず、ナビ項目のみを共通化する。
 */
export function GlobalNavMenu() {
  const pathname = usePathname() || ''
  const [open, setOpen] = useState(false)
  // AI面接のセッション画面表示中はFabが操作ボタンに重なるため隠す（StudentBottomNavと同じ理由、#1569）
  const interviewSessionActive = useInterviewSessionActive()

  if (pathname === '/' || !shouldShowStudentBottomNav(pathname)) return null
  if (interviewSessionActive) return null

  return (
    <>
      <Fab
        size="medium"
        color="primary"
        aria-label="メニューを開く"
        onClick={() => setOpen(true)}
        sx={{
          position: 'fixed',
          right: 16,
          bottom: { xs: 'calc(72px + env(safe-area-inset-bottom))', md: 16 },
          zIndex: (t) => t.zIndex.appBar,
        }}
      >
        <MenuIcon />
      </Fab>
      <Drawer anchor="right" open={open} onClose={() => setOpen(false)}>
        <Box sx={{ width: 280 }} role="presentation">
          <List component="nav" aria-label="画面の切り替え">
            {[...GLOBAL_NAV_HOME_ITEMS, ...SIDEBAR_NAV_ITEMS].map((item) => {
              // /interview と /interview/history のように前方一致が重なるときは、より長い方だけを現在地にする
              const current = isCurrentNav(pathname, item.href) &&
                ![...SIDEBAR_NAV_ITEMS].some((o) => o.href.length > item.href.length && isCurrentNav(pathname, o.href))
              return (
                <ListItem key={item.href} disablePadding>
                  <ListItemButton
                    component={Link}
                    // 結果画面は自己分析のセッションが要る。無ければチャットへ送る。
                    href={item.href === '/results' ? getResultsPathOrChat() : item.href}
                    prefetch
                    selected={current}
                    aria-current={current ? 'page' : undefined}
                    onClick={() => setOpen(false)}
                  >
                    <ListItemIcon sx={{ minWidth: 36 }}>{NAV_ICONS[item.href]}</ListItemIcon>
                    <ListItemText
                      primary={item.label}
                      secondary={current ? '表示中' : undefined}
                      primaryTypographyProps={{ fontWeight: current ? 700 : 400 }}
                    />
                  </ListItemButton>
                </ListItem>
              )
            })}
          </List>
        </Box>
      </Drawer>
    </>
  )
}
