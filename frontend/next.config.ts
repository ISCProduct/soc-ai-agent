import type { NextConfig } from 'next'

const isDev = process.env.NODE_ENV === 'development'

const securityHeaders = [
  {
    key: 'Content-Security-Policy',
    value: [
      "default-src 'self'",
      // 開発モードでは webpack が eval() を使うため 'unsafe-eval' が必要
      isDev
        ? "script-src 'self' 'unsafe-inline' 'unsafe-eval'"
        : "script-src 'self' 'unsafe-inline'",
      "style-src 'self' 'unsafe-inline'",
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
