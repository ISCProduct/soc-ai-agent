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
import FavoriteIcon from '@mui/icons-material/Favorite'
import MailOutlineIcon from '@mui/icons-material/MailOutline'
import ManageAccountsIcon from '@mui/icons-material/ManageAccounts'
import HomeIcon from '@mui/icons-material/Home'
import FactCheckIcon from '@mui/icons-material/FactCheck'
import QuizIcon from '@mui/icons-material/Quiz'
import SearchIcon from '@mui/icons-material/Search'
import { UI } from '@/lib/design/tokens'
import {
  GLOBAL_NAV_HOME_ITEMS,
  SIDEBAR_NAV_ITEMS,
  isCurrentNavExclusive,
  shouldShowStudentBottomNav,
} from '@/lib/sidebar-nav'
import { getResultsPathOrChat } from '@/lib/results-navigation'
import { useInterviewSessionActive } from '@/app/interview/sessionChrome'
import { useTranslations } from '@/lib/i18n'
import { useHasStoredUser } from './useHasStoredUser'

type NavHref = (typeof SIDEBAR_NAV_ITEMS)[number]['href'] | (typeof GLOBAL_NAV_HOME_ITEMS)[number]['href']

const NAV_ICONS: Record<NavHref, React.ReactNode> = {
  '/': <HomeIcon color="inherit" />,
  '/results': <FactCheckIcon color="inherit" />,
  '/correlation-diagram': <BorderAllIcon color="inherit" />,
  '/chat-history': <HistoryIcon color="inherit" />,
  '/resume': <DescriptionIcon color="inherit" />,
  '/assessments': <QuizIcon color="inherit" />,
  '/ability-assessments': <QuizIcon color="inherit" />,
  '/interview': <RecordVoiceOverIcon color="inherit" />,
  '/interview/history': <AssessmentIcon color="inherit" />,
  '/es-rewrite': <EditNoteIcon color="inherit" />,
  '/schedule': <CalendarMonthIcon color="inherit" />,
  '/favorites': <FavoriteIcon color="inherit" />,
  '/applications': <AssignmentIcon color="inherit" />,
  '/companies': <SearchIcon color="inherit" />,
  '/scout': <MailOutlineIcon color="inherit" />,
  '/profile': <ManageAccountsIcon color="inherit" />,
}

/**
 * ホーム画面以外のページから主要機能（ES・スケジュール・選考管理・相関図・面接履歴など）へ
 * 到達するための共通メニュー。ホームは HomeSidebar が同等の導線を持つため表示しない。
 * チャット進捗などホーム固有の責務は持たせず、ナビ項目のみを共通化する。
 */
export function GlobalNavMenu() {
  const pathname = usePathname() || ''
  const [open, setOpen] = useState(false)
  // AI面接のセッション画面表示中はFabが操作ボタンに重なるため隠す（StudentBottomNavと同じ理由、#1569）
  const interviewSessionActive = useInterviewSessionActive()
  const t = useTranslations('nav')
  // 未ログインの訪問者には出さない（下部ナビと同じ判定。#2222）。中の項目はどれも認証が要る画面
  const { mounted, hasStoredUser } = useHasStoredUser()

  if (pathname === '/' || !shouldShowStudentBottomNav(pathname)) return null
  if (interviewSessionActive) return null
  if (!mounted || !hasStoredUser) return null

  return (
    <>
      <Fab
        size="medium"
        color="primary"
        aria-label={t('menu.open')}
        onClick={() => setOpen(true)}
        sx={{
          position: 'fixed',
          // 印刷（ポートフォリオの PDF など）に載せない
          '@media print': { display: 'none' },
          right: 16,
          bottom: { xs: 'calc(72px + env(safe-area-inset-bottom))', md: 16 },
          zIndex: (t) => t.zIndex.appBar,
          boxShadow: 1,
          '&:hover': { boxShadow: 2 },
          '&:focus-visible': { outline: `2px solid ${UI.mark}`, outlineOffset: 3 },
        }}
      >
        <MenuIcon />
      </Fab>
      <Drawer
        anchor="right"
        open={open}
        onClose={() => setOpen(false)}
        sx={{ '& .MuiDrawer-paper': { bgcolor: UI.field, boxShadow: 2 } }}
      >
        <Box sx={{ width: 280 }} role="presentation">
          <List component="nav" aria-label={t('menu.label')}>
            {[...GLOBAL_NAV_HOME_ITEMS, ...SIDEBAR_NAV_ITEMS].map((item) => {
              // /interview と /interview/history のように前方一致が重なるときは、より長い方だけを現在地にする
              const current = isCurrentNavExclusive(pathname, item.href, SIDEBAR_NAV_ITEMS.map((o) => o.href))
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
                    sx={{
                      color: UI.inkSoft,
                      '&:hover': { bgcolor: UI.tint },
                      '&.Mui-selected, &.Mui-selected:hover': { bgcolor: UI.markTint, color: UI.mark },
                      '&.Mui-selected::before': {
                        content: '""',
                        position: 'absolute',
                        left: 0,
                        top: '50%',
                        transform: 'translateY(-50%)',
                        width: 4,
                        height: 24,
                        borderRadius: '2px',
                        bgcolor: UI.mark,
                      },
                      '&.Mui-focusVisible': { outline: `2px solid ${UI.mark}`, outlineOffset: -2 },
                    }}
                  >
                    <ListItemIcon sx={{ minWidth: 36, color: 'inherit' }}>{NAV_ICONS[item.href]}</ListItemIcon>
                    <ListItemText
                      primary={item.label}
                      secondary={current ? t('menu.current') : undefined}
                      primaryTypographyProps={{ fontWeight: current ? 700 : 400 }}
                      secondaryTypographyProps={{ color: UI.inkSoft }}
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
