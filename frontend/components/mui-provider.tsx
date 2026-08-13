'use client'

import { ThemeProvider, createTheme } from '@mui/material/styles'
import CssBaseline from '@mui/material/CssBaseline'
import { AppRouterCacheProvider } from '@mui/material-nextjs/v15-appRouter'

// ブランドカラー: サイドバー・ローディング画面・面接練習・ES添削で既に使われているオレンジに統一する
const BRAND_PRIMARY = '#ec5b13'

// app/globals.css で読み込んでいる日本語フォント。指定しないとMUIのCssBaselineが
// 既定のRoboto系にフォールバックし、globals.cssの指定を上書きしてしまう。
const FONT_FAMILY =
  "'Noto Sans JP', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Helvetica Neue', Arial, sans-serif"

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
  typography: {
    fontFamily: FONT_FAMILY,
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
