import { NextRequest, NextResponse } from 'next/server'
import {
  SERVER_BACKEND_URL,
  clearSessionCookies,
  setSessionCookies,
} from '@/lib/session-cookies'

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

  return NextResponse.json({
    user_id: Number(userId),
    user_token: userToken,
  })
}

export async function POST(request: NextRequest) {
  const { userId, userToken, refreshToken } = await request.json()
  if (!userId || !userToken) {
    return NextResponse.json({ error: 'userId and userToken are required' }, { status: 400 })
  }

  const response = NextResponse.json({ ok: true })
  setSessionCookies(response, String(userId), userToken, refreshToken)
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
