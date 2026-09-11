import { NextRequest, NextResponse } from 'next/server'
import {
  SERVER_BACKEND_URL,
  clearCompanySessionCookies,
  setCompanySessionCookies,
} from '@/lib/session-cookies'

// middleware が Cookie をリフレッシュしたあと、有効な company_user_token をクライアントへ返す。
// Backend 直叩き API は sessionStorage の JWT を使うため、Cookie と同期する (#1091, #616踏襲)。
export async function GET(request: NextRequest) {
  const companyUserId =
    request.headers.get('X-Company-User-ID') ||
    request.cookies.get('company_user_id')?.value
  const companyUserToken =
    request.headers.get('X-Company-User-Token') ||
    request.cookies.get('company_user_token')?.value

  if (!companyUserId || !companyUserToken) {
    return NextResponse.json({ error: 'unauthorized' }, { status: 401 })
  }

  return NextResponse.json({
    company_user_id: Number(companyUserId),
    company_user_token: companyUserToken,
  })
}

export async function POST(request: NextRequest) {
  const { companyUserId, companyUserToken, companyRefreshToken } = await request.json()
  if (!companyUserId || !companyUserToken) {
    return NextResponse.json(
      { error: 'companyUserId and companyUserToken are required' },
      { status: 400 }
    )
  }

  const response = NextResponse.json({ ok: true })
  setCompanySessionCookies(response, String(companyUserId), companyUserToken, companyRefreshToken)
  return response
}

export async function DELETE(request: NextRequest) {
  // Backend側でリフレッシュトークンを失効させてからCookieを削除する (#616踏襲)
  const refreshToken = request.cookies.get('company_refresh_token')?.value
  if (refreshToken) {
    try {
      await fetch(`${SERVER_BACKEND_URL}/api/company-auth/logout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      })
    } catch {
      // Backend不達でもCookie削除は続行する（トークンは期限切れで無効化される）
    }
  }

  const response = NextResponse.json({ ok: true })
  clearCompanySessionCookies(response)
  return response
}
