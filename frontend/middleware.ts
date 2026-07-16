import { NextRequest, NextResponse } from 'next/server'
import {
  SERVER_BACKEND_URL,
  setSessionCookies,
} from '@/lib/session-cookies'

// アクセストークンの残り有効期間がこの秒数を下回ったらリフレッシュする (#616)
const REFRESH_MARGIN_SECONDS = 120

interface RefreshedSession {
  userId: string
  userToken: string
  refreshToken: string
}

// JWTのexpを検証なしでデコードし、期限切れが近いか判定する
// （署名検証はBackendが行う。ここでは自動リフレッシュの契機判定のみ）
function tokenExpiresSoon(token: string): boolean {
  try {
    const payload = token.split('.')[1]
    if (!payload) return false
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/')
    const decoded: unknown = JSON.parse(atob(normalized))
    const exp = (decoded as { exp?: unknown }).exp
    if (typeof exp !== 'number') return false
    return exp - Date.now() / 1000 < REFRESH_MARGIN_SECONDS
  } catch {
    return false
  }
}

async function refreshSession(refreshToken: string): Promise<RefreshedSession | null> {
  try {
    const res = await fetch(`${SERVER_BACKEND_URL}/api/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })
    if (!res.ok) return null
    const data: { user_id?: number; user_token?: string; refresh_token?: string } =
      await res.json()
    if (!data.user_id || !data.user_token || !data.refresh_token) return null
    return {
      userId: String(data.user_id),
      userToken: data.user_token,
      refreshToken: data.refresh_token,
    }
  } catch {
    return null
  }
}

export async function middleware(request: NextRequest) {
  const userId = request.cookies.get('user_id')?.value
  const userToken = request.cookies.get('user_token')?.value
  const refreshToken = request.cookies.get('refresh_token')?.value

  const requestHeaders = new Headers(request.headers)
  let refreshed: RefreshedSession | null = null

  if (userId && userToken) {
    let effectiveUserId = userId
    let effectiveToken = userToken

    // アクセストークンの期限が近い場合は自動リフレッシュ（ユーザー操作を妨げない #616）
    if (refreshToken && tokenExpiresSoon(userToken)) {
      refreshed = await refreshSession(refreshToken)
      if (refreshed) {
        effectiveUserId = refreshed.userId
        effectiveToken = refreshed.userToken
      }
    }

    // httpOnly CookieからX-User-*ヘッダーを注入（クライアント送信ヘッダーを上書き）
    requestHeaders.set('X-User-ID', effectiveUserId)
    requestHeaders.set('X-User-Token', effectiveToken)
  }

  const response = NextResponse.next({
    request: { headers: requestHeaders },
  })

  // ローテーションされた新しいトークンペアをCookieへ反映
  if (refreshed) {
    setSessionCookies(response, refreshed.userId, refreshed.userToken, refreshed.refreshToken)
  }

  return response
}

export const config = {
  matcher: '/api/:path*',
}
