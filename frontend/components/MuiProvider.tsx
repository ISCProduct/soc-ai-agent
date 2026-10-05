'use client'

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { usePathname } from 'next/navigation'
import { ThemeProvider } from '@mui/material/styles'
import CssBaseline from '@mui/material/CssBaseline'
import { AppRouterCacheProvider } from '@mui/material-nextjs/v15-appRouter'
import { createAdminMuiTheme } from '@/lib/admin-theme'
import { createCompanyMuiTheme } from '@/lib/company-theme'
import {
  createStudentMuiTheme,
  readStudentThemeMode,
  writeStudentThemeMode,
  type StudentThemeMode,
} from '@/lib/student-theme'

// 管理画面は独立した identity を持つ（lib/admin-theme.ts に根拠を記載）。
// 以前は primary だけ指定した MUI 既定のままで、型階層・余白・角丸・
// コンポーネント指定が全て未定義だった。
const ADMIN_THEME = createAdminMuiTheme()
const COMPANY_THEME = createCompanyMuiTheme()

type StudentThemeContextValue = {
  mode: StudentThemeMode
  setMode: (mode: StudentThemeMode) => void
}

const StudentThemeContext = createContext<StudentThemeContextValue | null>(null)

export function useStudentTheme(): StudentThemeContextValue {
  const ctx = useContext(StudentThemeContext)
  if (!ctx) {
    return {
      mode: 'comfortable',
      setMode: () => {},
    }
  }
  return ctx
}

function storage(): Storage | null {
  if (typeof window === 'undefined') return null
  return window.localStorage
}

export function MuiProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname() || ''
  const isAdmin = pathname.startsWith('/admin')
  const isCompanyPortal = pathname.startsWith('/company-portal')
  const [mode, setModeState] = useState<StudentThemeMode>('comfortable')

  useEffect(() => {
    setModeState(readStudentThemeMode(storage()))
  }, [])

  const setMode = useCallback((next: StudentThemeMode) => {
    setModeState(next)
    writeStudentThemeMode(next, storage())
  }, [])

  const theme = useMemo(() => {
    if (isAdmin) return ADMIN_THEME
    if (isCompanyPortal) return COMPANY_THEME
    return createStudentMuiTheme(mode)
  }, [isAdmin, isCompanyPortal, mode])

  return (
    <AppRouterCacheProvider>
      <StudentThemeContext.Provider value={{ mode, setMode }}>
        <ThemeProvider theme={theme}>
          <CssBaseline />
          {children}
        </ThemeProvider>
      </StudentThemeContext.Provider>
    </AppRouterCacheProvider>
  )
}
