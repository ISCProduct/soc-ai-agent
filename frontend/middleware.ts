import { NextRequest, NextResponse } from 'next/server'
import {
  SERVER_BACKEND_URL,
  setSessionCookies,
  setCompanySessionCookies,
  clearSessionCookies,
  clearCompanySessionCookies,
} from '@/lib/auth/session-cookies'
import { extractTenantSlug, isAdminHost } from '@/lib/tenant'

// アクセストークンの残り有効期間がこの秒数を下回ったらリフレッシュする (#616)
const REFRESH_MARGIN_SECONDS = 120

// リクエストID (#1188)。FE -> BE -> RAG のログを1つのIDで突き合わせるため、
// 入口であるここで採番し、Route Handler 経由でBackendへ渡す。
const REQUEST_ID_HEADER = 'X-Request-ID'
const REQUEST_ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/

// クライアント指定値はログに出るため形式を検証し、不正なら採番し直す
function resolveRequestId(incoming: string | null): string {
  return incoming && REQUEST_ID_PATTERN.test(incoming) ? incoming : crypto.randomUUID()
}

// 企業相関図の旧URL。app/ 配下で唯一の大文字始まりセグメントだったため
// /correlation-diagram へ揃えた。既存のリンクやブックマークを切らさないよう
// ここで恒久リダイレクトする。
//
// next.config の redirects() を使わないのは、source の照合が大文字小文字を
// 区別せず、新URL自身もマッチして無限リダイレクトになるため(実測で確認)。
// ここでは pathname を厳密比較する。
const LEGACY_CORRELATION_DIAGRAM_PATH = '/Correlation-diagram'
const CORRELATION_DIAGRAM_PATH = '/correlation-diagram'

interface RefreshedSession {
  userId: string
  userToken: string
  refreshToken: string
}

interface RefreshedCompanySession {
  companyUserId: string
  companyUserToken: string
  companyRefreshToken: string
}

// リフレッシュの結果。失効が確定した失敗(401/403)と、一時的な障害
// (5xx・タイムアウト・通信断・応答JSONの不備)を区別する。
// 一時障害までセッション破棄に合流させると、Backendの再起動やDBの一時障害に
// 当たっただけの利用者が、まだ30日有効なリフレッシュトークンまで失って
// 再ログインを強制される。
type RefreshOutcome<T> =
  | { status: 'refreshed'; session: T }
  // リフレッシュトークンが失効・不正。Cookieを消して作り直させる
  | { status: 'expired' }
  // 一時的にリフレッシュできないだけ。Cookieは残して次回に賭ける
  | { status: 'unavailable' }

// 確定的な認証失敗か。Backendは失効・不正なリフレッシュトークンには401を返し
// (Backend/internal/controllers/auth/controller.go:305, 同company/auth_controller.go:97)、
// 内部エラーは5xxになる。何度試しても通らないものだけをセッション破棄の対象にする。
function isAuthFailureStatus(status: number): boolean {
  return status === 401 || status === 403
}

// JWTのexpを検証なしでデコードし、残り有効期間がmarginSecondsを下回るか判定する
// （署名検証はBackendが行う。ここでは自動リフレッシュの契機判定と、
//   期限切れトークンを後段へ流さない判定のみ）
function tokenExpiresSoon(token: string, marginSeconds = REFRESH_MARGIN_SECONDS): boolean {
  try {
    const payload = token.split('.')[1]
    if (!payload) return false
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/')
    const decoded: unknown = JSON.parse(atob(normalized))
    const exp = (decoded as { exp?: unknown }).exp
    if (typeof exp !== 'number') return false
    return exp - Date.now() / 1000 < marginSeconds
  } catch {
    return false
  }
}

// 後段(Route Handler / Server Component)へ渡すCookieから指定名を除く。
//
// X-User-* の注入を止めるだけでは足りない。Cookieを直接読む経路が残っており、
// 失効した値がそのまま認証情報として通ってしまう(#1519)。
//   - app/api/auth/session/route.ts:11-16 / app/api/company-auth/session/route.ts:11-16
//     ヘッダーが無ければCookieへフォールバックし、200で古いJWTを返す。
//     クライアントはそれをストレージへ再保存するため、直後のBackend呼び出しが401になる。
//   - lib/auth/server.ts:57-63 は next/headers の cookies() を直接読む。
//
// NextRequest#cookies.delete() は元リクエストのHeadersを書き換えるが、後段へ渡すのは
// 上で複製済みの requestHeaders なので届かない。複製側のcookieヘッダーを組み替える。
function dropRequestCookies(headers: Headers, names: readonly string[]): void {
  const cookie = headers.get('cookie')
  if (!cookie) return
  const kept = cookie.split(';').filter((pair) => !names.includes(pair.trim().split('=')[0]))
  if (kept.length === 0) {
    headers.delete('cookie')
    return
  }
  headers.set('cookie', kept.map((pair) => pair.trim()).join('; '))
}

async function refreshSession(refreshToken: string): Promise<RefreshOutcome<RefreshedSession>> {
  try {
    const res = await fetch(`${SERVER_BACKEND_URL}/api/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })
    if (!res.ok) {
      return { status: isAuthFailureStatus(res.status) ? 'expired' : 'unavailable' }
    }
    const data: { user_id?: number; user_token?: string; refresh_token?: string } =
      await res.json()
    if (!data.user_id || !data.user_token || !data.refresh_token) {
      return { status: 'unavailable' }
    }
    return {
      status: 'refreshed',
      session: {
        userId: String(data.user_id),
        userToken: data.user_token,
        refreshToken: data.refresh_token,
      },
    }
  } catch {
    // 通信断・タイムアウト・JSONパース失敗。再試行すれば通る可能性がある
    return { status: 'unavailable' }
  }
}

async function refreshCompanySession(
  companyRefreshToken: string,
): Promise<RefreshOutcome<RefreshedCompanySession>> {
  try {
    const res = await fetch(`${SERVER_BACKEND_URL}/api/company-auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: companyRefreshToken }),
    })
    if (!res.ok) {
      return { status: isAuthFailureStatus(res.status) ? 'expired' : 'unavailable' }
    }
    const data: { company_user_id?: number; token?: string; refresh_token?: string } =
      await res.json()
    if (!data.company_user_id || !data.token || !data.refresh_token) {
      return { status: 'unavailable' }
    }
    return {
      status: 'refreshed',
      session: {
        companyUserId: String(data.company_user_id),
        companyUserToken: data.token,
        companyRefreshToken: data.refresh_token,
      },
    }
  } catch {
    return { status: 'unavailable' }
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

  // 認証まわりの処理に入る前に返す（リダイレクトだけのためにトークン更新を走らせない）
  if (pathname === LEGACY_CORRELATION_DIAGRAM_PATH) {
    const redirectUrl = request.nextUrl.clone()
    redirectUrl.pathname = CORRELATION_DIAGRAM_PATH
    return NextResponse.redirect(redirectUrl, 308)
  }
  // admin.shukatsu-ai.jp は /admin 配下へ内部的にrewriteする(URLバーの表示は変えない)。
  // /api配下はNext.jsのAPI Route Handlerであり/admin/api/...という実体は存在しないため対象外。
  const needsAdminRewrite =
    isAdminHost(host) && !pathname.startsWith('/api') && !isUnderAdminPath(pathname)

  const userId = request.cookies.get('user_id')?.value
  const userToken = request.cookies.get('user_token')?.value
  const refreshToken = request.cookies.get('refresh_token')?.value

  const companyUserId = request.cookies.get('company_user_id')?.value
  const companyUserToken = request.cookies.get('company_user_token')?.value
  const companyRefreshToken = request.cookies.get('company_refresh_token')?.value

  const requestHeaders = new Headers(request.headers)
  // クライアントが直接送ってきた可能性のあるなりすましヘッダーを必ず除去してから、
  // 以降で信頼できる情報源(httpOnly Cookie / サブドメイン解決)からのみ設定し直す。
  // cookie不在やテナント未解決の場合にクライアント指定値がそのまま後段へ通過していた(#987)。
  requestHeaders.delete('X-User-ID')
  requestHeaders.delete('X-User-Token')
  requestHeaders.delete('X-Company-User-ID')
  requestHeaders.delete('X-Company-User-Token')
  requestHeaders.delete('X-Tenant-Slug')
  let refreshed: RefreshedSession | null = null
  // リフレッシュが401/403で失敗した=失効が確定したセッション。Cookieを消して
  // 作り直させる(#1519)。一時障害(5xx・通信断)はここに含めない
  let sessionExpired = false
  let companySessionExpired = false
  let companyRefreshed: RefreshedCompanySession | null = null

  // 学園サブドメイン(<学園slug>.shukatsu-ai.jp)をBackendへ引き継ぐ
  const tenantSlug = extractTenantSlug(request.headers.get('host') ?? '')
  if (tenantSlug) requestHeaders.set('X-Tenant-Slug', tenantSlug)

  const requestId = resolveRequestId(request.headers.get(REQUEST_ID_HEADER))
  requestHeaders.set(REQUEST_ID_HEADER, requestId)

  if (userId && userToken) {
    let effectiveUserId = userId
    let effectiveToken = userToken

    // アクセストークンの期限が近い場合は自動リフレッシュ（ユーザー操作を妨げない #616）
    if (refreshToken && tokenExpiresSoon(userToken)) {
      const outcome = await refreshSession(refreshToken)
      if (outcome.status === 'refreshed') {
        refreshed = outcome.session
        effectiveUserId = refreshed.userId
        effectiveToken = refreshed.userToken
      } else if (outcome.status === 'expired') {
        // リフレッシュトークンの失効が確定したのでセッションを捨てる(#1519)。
        //
        // 以前はここで何もせず、期限切れのトークンをそのままヘッダーへ注入し
        // Cookie も残していた。結果、ページが 401 を受けてログイン画面へ飛び、
        // そのログイン画面でも Cookie が残っているため再びリフレッシュを試み、
        // ログイン画面とローディングを無限に往復した。利用者は Cookie を
        // 消す方法を知らないので自力で復帰できない。
        //
        // リフレッシュトークンの有効期間は30日なので、30日以上ぶりに
        // アクセスした利用者は全員これに入る。
        sessionExpired = true
      }
      // 'unavailable'(5xx・通信断・タイムアウト)はCookieを残す。一時障害で消すと
      // 通信が不安定なだけの利用者を毎回ログアウトさせてしまう。ただし下の判定で
      // 期限切れトークンは後段へ渡さないので、未ログイン扱いで描画されループしない。
    }

    if (sessionExpired || tokenExpiresSoon(effectiveToken, 0)) {
      // 期限切れトークンを後段へ渡すとBackendが401を返し、ページがログイン画面へ
      // 飛び、そのログイン画面でも同じCookieで同じことを繰り返す(#1519)。
      // X-User-* を注入しないだけでなく、Cookie経由の抜け道も閉じる。
      dropRequestCookies(requestHeaders, ['user_id', 'user_token'])
    } else {
      // httpOnly CookieからX-User-*ヘッダーを注入（クライアント送信ヘッダーを上書き）
      requestHeaders.set('X-User-ID', effectiveUserId)
      requestHeaders.set('X-User-Token', effectiveToken)
    }
  }

  if (companyUserId && companyUserToken) {
    let effectiveCompanyUserId = companyUserId
    let effectiveCompanyToken = companyUserToken

    // アクセストークンの期限が近い場合は自動リフレッシュ（ユーザー操作を妨げない #1091, #616踏襲）
    if (companyRefreshToken && tokenExpiresSoon(companyUserToken)) {
      const outcome = await refreshCompanySession(companyRefreshToken)
      if (outcome.status === 'refreshed') {
        companyRefreshed = outcome.session
        effectiveCompanyUserId = companyRefreshed.companyUserId
        effectiveCompanyToken = companyRefreshed.companyUserToken
      } else if (outcome.status === 'expired') {
        // 企業ポータル側も同じ構造。学生側だけ直すと企業ユーザーが同じループに残る。
        companySessionExpired = true
      }
    }

    if (companySessionExpired || tokenExpiresSoon(effectiveCompanyToken, 0)) {
      dropRequestCookies(requestHeaders, ['company_user_id', 'company_user_token'])
    } else {
      requestHeaders.set('X-Company-User-ID', effectiveCompanyUserId)
      requestHeaders.set('X-Company-User-Token', effectiveCompanyToken)
    }
  }

  let response: NextResponse
  if (needsAdminRewrite) {
    const url = request.nextUrl.clone()
    url.pathname = `/admin${pathname}`
    response = NextResponse.rewrite(url, { request: { headers: requestHeaders } })
  } else {
    response = NextResponse.next({ request: { headers: requestHeaders } })
  }

  // 問い合わせ時にユーザーがIDを提示できるよう、ブラウザ側にも返す(#1188)
  response.headers.set(REQUEST_ID_HEADER, requestId)

  // ローテーションされた新しいトークンペアをCookieへ反映
  if (refreshed) {
    setSessionCookies(response, refreshed.userId, refreshed.userToken, refreshed.refreshToken)
  }
  // 企業ポータル側も同様に書き戻す。これが無いとリフレッシュトークンの
  // ローテーション後に古い値がCookieへ残り、企業ユーザーがログアウトされる (#1091の取りこぼし)
  if (companyRefreshed) {
    setCompanySessionCookies(
      response,
      companyRefreshed.companyUserId,
      companyRefreshed.companyUserToken,
      companyRefreshed.companyRefreshToken,
    )
  }

  // 失効が確定したセッションのCookieは消す。残すと次のリクエストでも同じ
  // リフレッシュを試み、ログイン画面とローディングを往復し続ける(#1519)。
  if (sessionExpired) {
    clearSessionCookies(response)
  }
  if (companySessionExpired) {
    clearCompanySessionCookies(response)
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
