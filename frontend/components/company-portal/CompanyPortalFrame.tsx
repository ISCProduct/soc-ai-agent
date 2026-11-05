'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { Box, Button, Menu, MenuItem } from '@mui/material'
import { companyAuthService } from '@/lib/company/auth'
import { reviewMark } from '@/lib/company/marks'
import { COMPANY_NAME_CHANGED_EVENT, companyProfileService } from '@/lib/company/profile'
import { MarkLabel } from '@/components/company-portal/MarkLabel'

const HIDDEN_PREFIXES = [
  '/company-portal/sign-in',
  '/company-portal/register',
  '/company-portal/forgot-password',
  '/company-portal/reset-password',
  '/company-portal/setup',
]

const NAV = [
  { href: '/company-portal', label: 'ホーム' },
  { href: '/company-portal/jobs', label: '求人' },
  { href: '/company-portal/applications', label: '応募者' },
  { href: '/company-portal/students', label: '学生を探す' },
  { href: '/company-portal/scouts', label: 'スカウト' },
] as const

function showNav(pathname: string): boolean {
  if (!pathname.startsWith('/company-portal')) return false
  return !HIDDEN_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`))
}

function isActive(pathname: string, href: string): boolean {
  if (href === '/company-portal') return pathname === href
  if (href === '/company-portal/scouts') {
    return pathname === href || pathname.startsWith('/company-portal/scout')
  }
  return pathname === href || pathname.startsWith(`${href}/`)
}

export function CompanyPortalFrame({ children }: { children: React.ReactNode }) {
  const pathname = usePathname() || ''
  const router = useRouter()
  const [ready, setReady] = useState(false)
  const [companyName, setCompanyName] = useState('')
  const [reviewed, setReviewed] = useState<boolean | null>(null)
  const [userName, setUserName] = useState('')
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null)

  useEffect(() => {
    setReady(true)
  }, [])

  useEffect(() => {
    if (!ready || !showNav(pathname)) return
    const stored = companyAuthService.getStoredUser()
    if (!stored) return
    setUserName(stored.name)
    companyProfileService
      .get()
      .then((profile) => {
        setCompanyName(profile.name)
        setReviewed(profile.data_status === 'published')
      })
      .catch(() => {
        setCompanyName('')
        setReviewed(null)
      })
  }, [ready, pathname])

  useEffect(() => {
    const onNameChanged = (event: Event) => {
      const name = (event as CustomEvent<string>).detail
      if (typeof name === 'string' && name.trim()) setCompanyName(name)
    }
    window.addEventListener(COMPANY_NAME_CHANGED_EVENT, onNameChanged)
    return () => window.removeEventListener(COMPANY_NAME_CHANGED_EVENT, onNameChanged)
  }, [])

  if (!showNav(pathname) || !ready) {
    return <>{children}</>
  }

  const review = reviewed === null ? null : reviewMark(reviewed)

  return (
    <>
      <Box
        component="header"
        sx={{ bgcolor: 'background.paper', borderBottom: '1px solid', borderColor: 'divider' }}
      >
        <Box
          sx={{
            maxWidth: 1080,
            mx: 'auto',
            px: { xs: 2, sm: 3 },
            py: 1.5,
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            gap: { xs: 1, md: 3 },
          }}
        >
          <Box sx={{ minWidth: 160 }}>
            <Box component="p" sx={{ m: 0, fontWeight: 700, fontSize: '1rem' }}>
              {companyName || '企業ポータル'}
            </Box>
            {review && <MarkLabel mark={review.mark} label={review.label} tone={review.tone} />}
          </Box>
          <Box
            component="nav"
            aria-label="企業ポータル"
            sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, flex: 1 }}
          >
            {NAV.map((item) => {
              const active = isActive(pathname, item.href)
              return (
                <Box
                  key={item.href}
                  component={Link}
                  href={item.href}
                  aria-current={active ? 'page' : undefined}
                  sx={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    minHeight: 44,
                    px: 1.25,
                    textDecoration: 'none',
                    color: 'text.primary',
                    fontWeight: active ? 700 : 500,
                    fontSize: '1rem',
                    borderBottom: '3px solid',
                    borderColor: active ? 'primary.main' : 'transparent',
                  }}
                >
                  {item.label}
                </Box>
              )
            })}
          </Box>
          <Button
            variant="text"
            color="inherit"
            aria-haspopup="menu"
            aria-expanded={menuAnchor ? 'true' : undefined}
            onClick={(event) => setMenuAnchor(event.currentTarget)}
          >
            {userName || 'アカウント'} ▾
          </Button>
          <Menu
            anchorEl={menuAnchor}
            open={Boolean(menuAnchor)}
            onClose={() => setMenuAnchor(null)}
          >
            <MenuItem
              onClick={() => {
                setMenuAnchor(null)
                router.push('/company-portal/settings')
              }}
            >
              設定
            </MenuItem>
            <MenuItem
              onClick={() => {
                setMenuAnchor(null)
                companyAuthService.logout()
                router.push('/company-portal/sign-in')
              }}
            >
              ログアウト
            </MenuItem>
          </Menu>
        </Box>
      </Box>
      {children}
    </>
  )
}
