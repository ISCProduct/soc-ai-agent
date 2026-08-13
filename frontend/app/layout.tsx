import './globals.css'
import type { Metadata } from 'next'
import { MuiProvider } from '@/components/mui-provider'
import { Analytics } from '@vercel/analytics/react'
// Vercelでホスティングされるまで実データは収集されない(AWS ECSデプロイのため現状は未計測)
import { SpeedInsights } from '@vercel/speed-insights/next'

export const metadata: Metadata = {
  title: 'IT企業エージェント',
  description: 'AI-powered IT Career Agent',
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
        </MuiProvider>
        <Analytics />
        <SpeedInsights />
      </body>
    </html>
  )
}
