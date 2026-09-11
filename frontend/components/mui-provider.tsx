'use client'

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { usePathname } from 'next/navigation'
import { ThemeProvider, createTheme } from '@mui/material/styles'
import CssBaseline from '@mui/material/CssBaseline'
import { AppRouterCacheProvider } from '@mui/material-nextjs/v15-appRouter'
import {
  createStudentMuiTheme,
  readStudentThemeMode,
  writeStudentThemeMode,
  type StudentThemeMode,
} from '@/lib/student-theme'

const ADMIN_THEME = createTheme({
  palette: { mode: 'light', primary: { main: '#1976d2' } },
})

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
  const [mode, setModeState] = useState<StudentThemeMode>('comfortable')

  useEffect(() => {
    setModeState(readStudentThemeMode(storage()))
  }, [])

  const setMode = useCallback((next: StudentThemeMode) => {
    setModeState(next)
    writeStudentThemeMode(next, storage())
  }, [])

  const theme = useMemo(
    () => (isAdmin ? ADMIN_THEME : createStudentMuiTheme(mode)),
    [isAdmin, mode],
  )

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
