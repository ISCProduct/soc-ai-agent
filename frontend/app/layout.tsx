import './globals.css'
import type { Metadata } from 'next'
import { MuiProvider } from '@/components/mui-provider'
import { StudentBottomNav } from '@/components/student-bottom-nav'
import { Analytics } from '@vercel/analytics/react'
// Vercelでホスティングされるまで実データは収集されない(AWS ECSデプロイのため現状は未計測)
import { SpeedInsights } from '@vercel/speed-insights/next'

const TITLE = 'IT企業エージェント | 就活AI'
const DESCRIPTION = '適性診断から企業マッチングまで。IT就活を支援するAIエージェントです。'

export const metadata: Metadata = {
  metadataBase: new URL(process.env.NEXT_PUBLIC_SITE_URL || 'https://shukatsu-ai.jp'),
  title: TITLE,
  description: DESCRIPTION,
  openGraph: {
    title: TITLE,
    description: DESCRIPTION,
    locale: 'ja_JP',
    type: 'website',
    siteName: '就活AI',
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
    <html lang="ja">
      <head>
        <meta charSet="UTF-8" />
      </head>
      <body style={{ margin: 0, padding: 0 }}>
        <MuiProvider>
          {children}
          <StudentBottomNav />
        </MuiProvider>
        <Analytics />
        <SpeedInsights />
      </body>
    </html>
  )
}
