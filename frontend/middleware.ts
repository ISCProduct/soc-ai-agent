import { NextRequest, NextResponse } from 'next/server'
import {
  SERVER_BACKEND_URL,
  setSessionCookies,
} from '@/lib/session-cookies'
import { extractTenantSlug, isAdminHost } from '@/lib/tenant'

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

// pathnameが既に /admin 配下(自身を含む)かどうか。前方一致だと /administrator 等の
// 無関係なパスまで「rewrite不要」と誤判定してしまうため、セグメント単位で比較する。
function isUnderAdminPath(pathname: string): boolean {
  return pathname === '/admin' || pathname.startsWith('/admin/')
}

export async function middleware(request: NextRequest) {
  const host = request.headers.get('host') ?? ''
  const pathname = request.nextUrl.pathname
  // admin.shukatsu-ai.jp は /admin 配下へ内部的にrewriteする(URLバーの表示は変えない)。
  // /api配下はNext.jsのAPI Route Handlerであり/admin/api/...という実体は存在しないため対象外。
  const needsAdminRewrite =
    isAdminHost(host) && !pathname.startsWith('/api') && !isUnderAdminPath(pathname)

  const userId = request.cookies.get('user_id')?.value
  const userToken = request.cookies.get('user_token')?.value
  const refreshToken = request.cookies.get('refresh_token')?.value

  const requestHeaders = new Headers(request.headers)
  // クライアントが直接送ってきた可能性のあるなりすましヘッダーを必ず除去してから、
  // 以降で信頼できる情報源(httpOnly Cookie / サブドメイン解決)からのみ設定し直す。
  // cookie不在やテナント未解決の場合にクライアント指定値がそのまま後段へ通過していた(#987)。
  requestHeaders.delete('X-User-ID')
  requestHeaders.delete('X-User-Token')
  requestHeaders.delete('X-Tenant-Slug')
  let refreshed: RefreshedSession | null = null

  // 学園サブドメイン(<学園slug>.shukatsu-ai.jp)をBackendへ引き継ぐ
  const tenantSlug = extractTenantSlug(request.headers.get('host') ?? '')
  if (tenantSlug) requestHeaders.set('X-Tenant-Slug', tenantSlug)

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

  let response: NextResponse
  if (needsAdminRewrite) {
    const url = request.nextUrl.clone()
    url.pathname = `/admin${pathname}`
    response = NextResponse.rewrite(url, { request: { headers: requestHeaders } })
  } else {
    response = NextResponse.next({ request: { headers: requestHeaders } })
  }

  // ローテーションされた新しいトークンペアをCookieへ反映
  if (refreshed) {
    setSessionCookies(response, refreshed.userId, refreshed.userToken, refreshed.refreshToken)
  }

  return response
}

export const config = {
  // admin.shukatsu-ai.jpのrewriteはページ遷移でも必要なため、静的アセットを除く全パスに
  // マッチさせる。public/配下の拡張子付きファイル(3Dモデル・画像等)も対象外にする。
  matcher: [
    '/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp|ico|glb|gltf|mp3|mp4|woff|woff2)$).*)',
  ],
}
