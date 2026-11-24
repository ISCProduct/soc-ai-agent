import { NextResponse } from 'next/server'
import { setAdminTokenCookie, setCompanySessionCookies, setSessionCookies } from './session-cookies'

const TOKEN_FIELDS = ['token', 'user_token', 'refresh_token'] as const

function withoutTokens(data: Record<string, unknown>): Record<string, unknown> {
  const rest = { ...data }
  for (const field of TOKEN_FIELDS) delete rest[field]
  return rest
}

/**
 * ログイン・登録の応答（Backend の JSON）から学生のセッション Cookie を張り、
 * トークンを取り除いた本文を画面へ返す（SOCAIAGENT-683）。トークンが無ければそのまま返す。
 */
export function respondWithStudentSession(data: Record<string, unknown>, status: number): NextResponse {
  const response = NextResponse.json(withoutTokens(data), { status })
  if (!data.user_id || typeof data.user_token !== 'string' || data.user_token === '') return response
  setSessionCookies(
    response,
    String(data.user_id),
    data.user_token,
    typeof data.refresh_token === 'string' ? data.refresh_token : undefined,
  )
  if (typeof data.token === 'string' && data.token !== '') {
    setAdminTokenCookie(response, data.token)
  } else {
    // 前に同じブラウザで管理者が使っていた admin_token を残さない
    response.cookies.delete('admin_token')
  }
  return response
}

/** 企業ユーザー版。Backend の応答では token がアクセストークン。 */
export function respondWithCompanySession(data: Record<string, unknown>, status: number): NextResponse {
  const response = NextResponse.json(withoutTokens(data), { status })
  if (!data.company_user_id || typeof data.token !== 'string' || data.token === '') return response
  setCompanySessionCookies(
    response,
    String(data.company_user_id),
    data.token,
    typeof data.refresh_token === 'string' ? data.refresh_token : undefined,
  )
  return response
}
