import type { NextConfig } from 'next'

const isDev = process.env.NODE_ENV === 'development'

const securityHeaders = [
  {
    key: 'Content-Security-Policy',
    value: [
      "default-src 'self'",
      // 開発モードでは webpack が eval() を使うため 'unsafe-eval' が必要
      isDev
        ? "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://va.vercel-scripts.com"
        : "script-src 'self' 'unsafe-inline' https://va.vercel-scripts.com",
      "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
      "font-src 'self' https://fonts.gstatic.com",
      "img-src 'self' data: https:",
      // blob: URL の音声/動画再生を許可（AI面接の TTS 音声は blob: URL 経由で再生）
      "media-src 'self' blob:",
      // 開発モードでは webpack HMR の WebSocket 接続を許可
      isDev
        ? "connect-src 'self' blob: http://localhost:* https://api.openai.com ws://localhost:* wss://localhost:*"
        : "connect-src 'self' blob: https://api.openai.com",
      "frame-ancestors 'none'",
    ].join('; '),
  },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
  { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
]

// 面接ページはカメラ・マイクへのアクセスが必要なため Permissions-Policy を上書き
const interviewPermissionsHeader = {
  key: 'Permissions-Policy',
  value: 'camera=(self), microphone=(self), geolocation=()',
}

const nextConfig: NextConfig = {
  reactStrictMode: true,
  output: 'standalone',
  // zod v4 は ESM-first ("type":"module") のため Webpack が解決できない場合がある
  transpilePackages: ['zod'],
  // MUI emotion CSS-in-JS のSSR対応
  compiler: {
    emotion: true,
  },
  async headers() {
    return [
      {
        source: '/(.*)',
        headers: securityHeaders,
      },
      {
        // 面接ページのみカメラ・マイクを許可（他ページは securityHeaders で引き続き禁止）
        source: '/interview(.*)',
        headers: [
          ...securityHeaders.filter((h) => h.key !== 'Permissions-Policy'),
          interviewPermissionsHeader,
        ],
      },
    ]
  },
}

export default nextConfig
