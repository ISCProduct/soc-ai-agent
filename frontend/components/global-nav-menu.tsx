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
import ManageAccountsIcon from '@mui/icons-material/ManageAccounts'
import { SIDEBAR_NAV_ITEMS, shouldShowStudentBottomNav } from '@/lib/sidebar-nav'

const NAV_ICONS: Record<(typeof SIDEBAR_NAV_ITEMS)[number]['href'], React.ReactNode> = {
  '/Correlation-diagram': <BorderAllIcon color="primary" />,
  '/chat-history': <HistoryIcon color="primary" />,
  '/resume': <DescriptionIcon color="primary" />,
  '/interview': <RecordVoiceOverIcon color="primary" />,
  '/interview/history': <AssessmentIcon color="primary" />,
  '/es-rewrite': <EditNoteIcon color="primary" />,
  '/schedule': <CalendarMonthIcon color="primary" />,
  '/applications': <AssignmentIcon color="primary" />,
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

  if (pathname === '/' || !shouldShowStudentBottomNav(pathname)) return null

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
          <List>
            {SIDEBAR_NAV_ITEMS.map((item) => (
              <ListItem key={item.href} disablePadding>
                <ListItemButton component={Link} href={item.href} prefetch onClick={() => setOpen(false)}>
                  <ListItemIcon sx={{ minWidth: 36 }}>{NAV_ICONS[item.href]}</ListItemIcon>
                  <ListItemText primary={item.label} />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </Box>
      </Drawer>
    </>
  )
}
