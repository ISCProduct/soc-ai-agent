import { NextRequest, NextResponse } from 'next/server'
import {
  SERVER_BACKEND_URL,
  clearSessionCookies,
  setAdminTokenCookie,
  setSessionCookies,
} from '@/lib/auth/session-cookies'

// middleware が Cookie をリフレッシュしたあと、有効な user_token をクライアントへ返す。
// 面接など Backend 直叩き API は sessionStorage の JWT を使うため、Cookie と同期する (#616)。
export async function GET(request: NextRequest) {
  const userId =
    request.headers.get('X-User-ID') ||
    request.cookies.get('user_id')?.value
  const userToken =
    request.headers.get('X-User-Token') ||
    request.cookies.get('user_token')?.value

  if (!userId || !userToken) {
    return NextResponse.json({ error: 'unauthorized' }, { status: 401 })
  }

  // middleware がこのリクエストで取り直した値を優先する。Cookie は更新前のまま届く。
  const adminToken =
    request.headers.get('X-Refreshed-Admin-Token') ||
    request.cookies.get('admin_token')?.value

  return NextResponse.json({
    user_id: Number(userId),
    user_token: userToken,
    ...(adminToken ? { token: adminToken } : {}),
  })
}

export async function POST(request: NextRequest) {
  const { userId, userToken, refreshToken, adminToken } = await request.json()
  if (!userId || !userToken) {
    return NextResponse.json({ error: 'userId and userToken are required' }, { status: 400 })
  }

  const response = NextResponse.json({ ok: true })
  setSessionCookies(response, String(userId), userToken, refreshToken)
  if (typeof adminToken === 'string' && adminToken !== '') {
    setAdminTokenCookie(response, adminToken)
  } else {
    // 管理者の次に同じブラウザで別の利用者が入ると、前の管理者トークンが最大30日残る。
    response.cookies.delete('admin_token')
  }
  return response
}

export async function DELETE(request: NextRequest) {
  // Backend側でリフレッシュトークンを失効させてからCookieを削除する (#616)
  const refreshToken = request.cookies.get('refresh_token')?.value
  if (refreshToken) {
    try {
      await fetch(`${SERVER_BACKEND_URL}/api/auth/logout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      })
    } catch {
      // Backend不達でもCookie削除は続行する（トークンは期限切れで無効化される）
    }
  }

  const response = NextResponse.json({ ok: true })
  clearSessionCookies(response)
  return response
}
