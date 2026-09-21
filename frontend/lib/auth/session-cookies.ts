import { NextResponse } from 'next/server'

// サーバーサイド（middleware / Route Handler）から Backend を呼ぶ際のURL
// コンテナ間通信では BACKEND_URL（例: http://app:8080）を優先する
export const SERVER_BACKEND_URL =
  process.env.BACKEND_URL || process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:8080'

// リフレッシュトークンの有効期間（30日）に合わせる (#616)
export const SESSION_COOKIE_OPTIONS = {
  httpOnly: true,
  secure: process.env.NODE_ENV === 'production',
  sameSite: 'strict' as const,
  path: '/',
  maxAge: 60 * 60 * 24 * 30,
}

// セッションCookie（user_id / user_token / refresh_token）を設定する
export function setSessionCookies(
  response: NextResponse,
  userId: string,
  userToken: string,
  refreshToken?: string
): void {
  response.cookies.set('user_id', userId, SESSION_COOKIE_OPTIONS)
  response.cookies.set('user_token', userToken, SESSION_COOKIE_OPTIONS)
  if (refreshToken) {
    response.cookies.set('refresh_token', refreshToken, SESSION_COOKIE_OPTIONS)
  }
}

// セッションCookieをすべて削除する
export function clearSessionCookies(response: NextResponse): void {
  response.cookies.delete('user_id')
  response.cookies.delete('user_token')
  response.cookies.delete('refresh_token')
}

// 企業ユーザー用セッションCookie（company_user_id / company_user_token / company_refresh_token）を設定する（#1091）
export function setCompanySessionCookies(
  response: NextResponse,
  companyUserId: string,
  companyUserToken: string,
  companyRefreshToken?: string
): void {
  response.cookies.set('company_user_id', companyUserId, SESSION_COOKIE_OPTIONS)
  response.cookies.set('company_user_token', companyUserToken, SESSION_COOKIE_OPTIONS)
  if (companyRefreshToken) {
    response.cookies.set('company_refresh_token', companyRefreshToken, SESSION_COOKIE_OPTIONS)
  }
}

// 企業ユーザー用セッションCookieをすべて削除する
export function clearCompanySessionCookies(response: NextResponse): void {
  response.cookies.delete('company_user_id')
  response.cookies.delete('company_user_token')
  response.cookies.delete('company_refresh_token')
}
