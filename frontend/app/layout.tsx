import './globals.css'
import type { Metadata } from 'next'
import { MuiProvider } from '@/components/MuiProvider'
import { StudentBottomNav } from '@/components/StudentBottomNav'
import { GlobalNavMenu } from '@/components/GlobalNavMenu'
import { Analytics } from '@vercel/analytics/react'
// Vercelでホスティングされるまで実データは収集されない(AWS ECSデプロイのため現状は未計測)
import { SpeedInsights } from '@vercel/speed-insights/next'
import { SentryClientInit } from '@/components/SentryClientInit'
import { DEFAULT_LOCALE, createTranslator, htmlLang, ogLocale } from '@/lib/i18n'

// 文言は辞書（messages/ja.json の metadata）が正本。言語は lib/i18n の既定値から決める（SOCAIAGENT-595）。
const t = createTranslator('metadata', DEFAULT_LOCALE)
const TITLE = t('title')
const DESCRIPTION = t('description')

export const metadata: Metadata = {
  metadataBase: new URL(process.env.NEXT_PUBLIC_SITE_URL || 'https://shukatsu-ai.jp'),
  title: TITLE,
  description: DESCRIPTION,
  openGraph: {
    title: TITLE,
    description: DESCRIPTION,
    locale: ogLocale(DEFAULT_LOCALE),
    type: 'website',
    siteName: t('siteName'),
  },
  twitter: {
    card: 'summary_large_image',
    title: TITLE,
    description: DESCRIPTION,
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang={htmlLang(DEFAULT_LOCALE)} suppressHydrationWarning>
      <head>
        <meta charSet="UTF-8" />
      </head>
      <body>
        <SentryClientInit />
        <MuiProvider>
          {children}
          <GlobalNavMenu />
          <StudentBottomNav />
        </MuiProvider>
        <Analytics />
        <SpeedInsights />
      </body>
    </html>
  )
}
