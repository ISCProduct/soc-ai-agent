import { NextRequest, NextResponse } from 'next/server'
import {
  SERVER_BACKEND_URL,
  setAdminTokenCookie,
  setSessionCookies,
} from '@/lib/auth/session-cookies'
import { parseSessionPost } from '@/lib/auth/session-request'
import { clientIpHeaders } from '@/lib/api-proxy'
import { OAUTH_VERIFIER_COOKIE } from '@/lib/auth/oauth-verifier'

function fail(reason: string, status: number): NextResponse {
  // コード・verifier の値はログに出さない
  console.warn(`[oauth] exchange rejected reason=${reason}`)
  const response = NextResponse.json({ error: 'login_failed' }, { status })
  response.cookies.delete(OAUTH_VERIFIER_COOKIE)
  return response
}

// OAuth 完了時の短命コードを認証情報と交換し、セッション Cookie を張る (SOCAIAGENT-683)。
// verifier はこのブラウザでログインを始めたときの httpOnly Cookie にしか無い。
// 別のブラウザで始まったログインのコードを開いても、verifier が合わず Backend が拒否する。
// POST /api/auth/oauth/exchange  { code }
export async function POST(request: NextRequest) {
  const parsed = await parseSessionPost(request, 'oauth-exchange')
  if (!parsed.ok) return parsed.response
  const { code } = parsed.body
  if (typeof code !== 'string' || code === '') return fail('code', 400)

  const verifier = request.cookies.get(OAUTH_VERIFIER_COOKIE)?.value
  if (!verifier) return fail('verifier', 400)

  let res: Response
  try {
    res = await fetch(`${SERVER_BACKEND_URL}/api/auth/oauth/exchange`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        // IP 単位のレート制限(#1407)とリクエストIDの横断追跡(#1188)のため引き継ぐ
        ...clientIpHeaders(request),
        ...(request.headers.get('X-Request-ID') ? { 'X-Request-ID': request.headers.get('X-Request-ID') as string } : {}),
      },
      body: JSON.stringify({ code, verifier }),
      cache: 'no-store',
    })
  } catch {
    return fail('backend_unreachable', 502)
  }
  if (!res.ok) return fail(`backend_${res.status}`, 400)

  const data: unknown = await res.json().catch(() => null)
  if (typeof data !== 'object' || data === null) return fail('backend_body', 502)
  const auth = data as Record<string, unknown>
  if (!auth.user_id || typeof auth.user_token !== 'string' || auth.user_token === '') {
    return fail('backend_body', 502)
  }

  // トークンは Cookie にだけ置き、画面へは返さない
  const { token: _token, user_token: _userToken, refresh_token: _refreshToken, ...user } = auth
  const response = NextResponse.json(user)
  setSessionCookies(
    response,
    String(auth.user_id),
    auth.user_token,
    typeof auth.refresh_token === 'string' ? auth.refresh_token : undefined
  )
  if (typeof auth.token === 'string' && auth.token !== '') {
    setAdminTokenCookie(response, auth.token)
  } else {
    // 前に同じブラウザで管理者が使っていた admin_token を残さない
    response.cookies.delete('admin_token')
  }
  response.cookies.delete(OAUTH_VERIFIER_COOKIE)
  return response
}
