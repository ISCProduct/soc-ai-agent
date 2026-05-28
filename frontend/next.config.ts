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
      // 開発モードでは webpack HMR の WebSocket 接続を許可
      isDev
        ? "connect-src 'self' http://localhost:* https://api.openai.com ws://localhost:* wss://localhost:*"
        : "connect-src 'self' https://api.openai.com",
      "frame-ancestors 'none'",
    ].join('; '),
  },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
  { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
]

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
    ]
  },
}

export default nextConfig
