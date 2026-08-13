'use client'

import { ThemeProvider, createTheme } from '@mui/material/styles'
import CssBaseline from '@mui/material/CssBaseline'
import { AppRouterCacheProvider } from '@mui/material-nextjs/v15-appRouter'

// ブランドカラー: サイドバー・ローディング画面・面接練習・ES添削で既に使われているオレンジに統一する
const BRAND_PRIMARY = '#ec5b13'

const theme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: BRAND_PRIMARY,
    },
    secondary: {
      main: '#dc004e',
    },
  },
})

export function MuiProvider({ children }: { children: React.ReactNode }) {
  return (
    <AppRouterCacheProvider>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        {children}
      </ThemeProvider>
    </AppRouterCacheProvider>
  )
}
