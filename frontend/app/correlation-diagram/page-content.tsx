'use client'

import { Suspense, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import dynamic from 'next/dynamic'
import { Box, Button, Typography } from '@mui/material'
import Menu from '@mui/icons-material/Menu'
import ArrowBack from '@mui/icons-material/ArrowBack'
import { HomeSidebar } from '@/components/home/HomeSidebar'
import { NotificationBell } from '@/components/layout/NotificationBell'
import { useStudentTheme } from '@/components/providers/MuiProvider'
import { CompanyPicker } from '@/components/diagram/explorer/CompanyPicker'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import { PageLoading } from '@/components/common/PageLoading'
import { authService, type User } from '@/lib/auth'
import { UI } from '@/lib/design/tokens'

const CorrelationExplorer = dynamic(() => import('@/components/diagram/explorer/CorrelationExplorer'), {
  ssr: false, loading: () => <PageLoading message="企業相関図を準備しています…" />,
})

function CorrelationDiagramContent({ user }: { user: User }) {
  const router = useRouter()
  const searchParams = useSearchParams()
  const rawId = searchParams.get('company_id')
  const companyId = rawId && /^[1-9]\d*$/.test(rawId) && Number.isSafeInteger(Number(rawId)) ? Number(rawId) : null
  const [menuOpen, setMenuOpen] = useState(false)
  const { mode: displayMode } = useStudentTheme()
  function focusCompany(id: number | null, mode?: 'capital' | 'business') {
    const params = new URLSearchParams(searchParams.toString())
    if (id === null) params.delete('company_id'); else params.set('company_id', String(id))
    if (mode) params.set('diagram_type', mode)
    else params.delete('diagram_type')
    const href = `/correlation-diagram${params.size ? `?${params}` : ''}`
    // 一覧に戻る操作は履歴を増やさず、相関図との往復を防ぐ。
    if (id === null) router.replace(href, { scroll: false })
    else router.push(href, { scroll: false })
  }
  return <Box sx={{ height: '100dvh', display: 'flex', bgcolor: UI.field, color: UI.ink, overflow: 'hidden',
    ...(displayMode === 'comfortable' ? { '--ui-mark': '#0077ad', '--ui-paper': '#f5f8fb', '--ui-rule': '#cbd5e1', '--correlation-origin-bg': '#fff7de', '--correlation-origin-border': '#a66a00' } : {}),
    '& main .MuiButton-root': { color: UI.mark, borderRadius: '12px' }, '& main .MuiButton-contained': { bgcolor: UI.mark, color: '#fff' },
    '& main .MuiTabs-indicator': { bgcolor: UI.mark }, '& main .MuiTab-root.Mui-selected': { color: UI.mark },
  }}>
    <HomeSidebar user={user} onLogout={() => { authService.logout(); router.push('/login') }} mobileOpen={menuOpen} onMobileClose={() => setMenuOpen(false)} />
    <Box component="main" sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', overflowY: 'auto', px: { xs: 2, md: 4 }, pt: 1, pb: { xs: 0, md: 3 }, '& button': { minHeight: 44 } }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 1, mb: 1, minHeight: 48, flexShrink: 0 }}>
        <Button startIcon={<ArrowBack />} onClick={() => companyId ? focusCompany(null) : router.push('/')}>戻る</Button>
        {user ? <Box sx={{ display: { xs: 'flex', md: 'none' }, alignItems: 'center', gap: 1 }}>
          <Button variant="outlined" startIcon={<Menu />} onClick={() => setMenuOpen(true)} aria-expanded={menuOpen} sx={{ height: 44, px: 1.5, whiteSpace: 'nowrap' }}>メニュー</Button>
          <NotificationBell placement="inline" />
        </Box> : <Button component={Link} href="/">ホームへ</Button>}
      </Box>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1, mb: 1, flexShrink: 0 }}>
        <Typography component="h1" sx={{ fontSize: { xs: 24, md: 28 }, fontWeight: 700, lineHeight: 1.5 }}>企業相関図</Typography>
      </Box>
      <Typography sx={{ color: UI.inkSoft, mb: 2, display: { xs: 'none', sm: 'block' } }}>知っている会社から、まだ知らない会社へ。</Typography>
      {companyId === null ? <CompanyPicker onSelect={focusCompany} /> : <CorrelationExplorer companyId={companyId} onFocus={focusCompany} initialMode={searchParams.get('diagram_type') === 'business' ? 'business' : 'capital'} />}
      <Box sx={{ flexShrink: 0 }}><BottomNavSpacer /></Box>
    </Box>
  </Box>
}

export default function PageContent({ user }: { user: User }) {
  return <Suspense fallback={<PageLoading message="企業相関図を準備しています…" />}><CorrelationDiagramContent user={user} /></Suspense>
}
