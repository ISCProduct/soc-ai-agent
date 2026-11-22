import { NextRequest, NextResponse } from 'next/server'
import { BACKEND_URL } from '@/lib/backend-url'
import {
  OAUTH_VERIFIER_COOKIE,
  OAUTH_VERIFIER_COOKIE_OPTIONS,
  createOAuthVerifier,
  oauthBindOf,
} from '@/lib/auth/oauth-verifier'

const PROVIDERS = new Set(['google', 'github'])

// OAuth ログインの開始 (SOCAIAGENT-683)。verifier を httpOnly Cookie に入れ、
// そのハッシュ（bind）を付けて Backend の OAuth 開始へ送る。
// GET /api/auth/oauth/start?provider=google&tenant=<slug>
export async function GET(request: NextRequest) {
  const provider = request.nextUrl.searchParams.get('provider') ?? ''
  if (!PROVIDERS.has(provider)) {
    return NextResponse.json({ error: 'unknown provider' }, { status: 400 })
  }

  const verifier = createOAuthVerifier()
  const target = new URL(`${BACKEND_URL}/api/auth/${provider}`)
  const tenant = request.nextUrl.searchParams.get('tenant')
  if (tenant) target.searchParams.set('tenant', tenant)
  target.searchParams.set('bind', oauthBindOf(verifier))

  const response = NextResponse.redirect(target, 307)
  response.cookies.set(OAUTH_VERIFIER_COOKIE, verifier, OAUTH_VERIFIER_COOKIE_OPTIONS)
  return response
}
