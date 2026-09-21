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
          />
        ))}
      </BottomNavigation>
    </Paper>
  )
}
