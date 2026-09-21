'use client'

import { usePathname, useRouter } from 'next/navigation'
import { BottomNavigation, BottomNavigationAction, Paper } from '@mui/material'
import ChatIcon from '@mui/icons-material/Chat'
import BusinessIcon from '@mui/icons-material/Business'
import RecordVoiceOverIcon from '@mui/icons-material/RecordVoiceOver'
import DescriptionIcon from '@mui/icons-material/Description'
import ManageAccountsIcon from '@mui/icons-material/ManageAccounts'
import { STUDENT_BOTTOM_NAV_ITEMS, shouldShowStudentBottomNav } from '@/lib/sidebar-nav'
import { getResultsPathOrChat } from '@/lib/results-navigation'

const ICONS = {
  '/': <ChatIcon />,
  '/results': <BusinessIcon />,
  '/interview': <RecordVoiceOverIcon />,
  '/resume': <DescriptionIcon />,
  '/profile': <ManageAccountsIcon />,
} as const

export function StudentBottomNav() {
  const pathname = usePathname() || ''
  const router = useRouter()
  if (!shouldShowStudentBottomNav(pathname)) return null

  const current =
    STUDENT_BOTTOM_NAV_ITEMS.find((i) =>
      i.href === '/' ? pathname === '/' : pathname === i.href || pathname.startsWith(`${i.href}/`),
    )?.href ?? false

  return (
    <Paper
      elevation={8}
      sx={{
        position: 'fixed',
        bottom: 0,
        left: 0,
        right: 0,
        zIndex: (t) => t.zIndex.appBar,
        display: { xs: 'block', md: 'none' },
        pb: 'env(safe-area-inset-bottom)',
      }}
    >
      <BottomNavigation
        showLabels
        value={current}
        onChange={(_, href: string) => {
          router.push(href === '/results' ? getResultsPathOrChat() : href)
        }}
      >
        {STUDENT_BOTTOM_NAV_ITEMS.map((item) => (
          <BottomNavigationAction
            key={item.href}
            value={item.href}
            label={item.label}
            icon={ICONS[item.href]}
            sx={{
              // MUI既定の minWidth=80px は5項目で400px必要になり、
              // 400px未満の端末で両端が画面外へ出る（320pxでは診断が x=-40、
              // 設定の右端が 360 まで押し出されて押せない）。
              // 0 にして等分させる。文字は隠さない。
              minWidth: 0,
              px: 0.5,
              '& .MuiBottomNavigationAction-label': {
                // 等分すると320pxでは1項目64px。既定12pxだと「履歴書」が折り返すため
                // わずかに縮める。読めなくなる大きさにはしない。
                fontSize: { xs: '0.6875rem', sm: '0.75rem' },
                whiteSpace: 'nowrap',
              },
            }}
          />
        ))}
      </BottomNavigation>
    </Paper>
  )
}
