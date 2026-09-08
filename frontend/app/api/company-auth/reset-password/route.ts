import { NextRequest, NextResponse } from 'next/server'
import { setCompanySessionCookies } from '@/lib/session-cookies'
import type { CompanyAuthResponse } from '@/lib/company-auth'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

// パスワード再設定を実行する（認証不要）。成功時はAcceptInviteと同じAuthResponseが返るため、
// そのままログイン状態にする httpOnly Cookie を設定する (#1196)
export async function POST(request: NextRequest) {
  const body = await request.text()
  const res = await fetch(`${BACKEND_URL}/api/company-auth/reset-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  })
  const text = await res.text()
  const response = new NextResponse(text, {
    status: res.status,
    headers: { 'Content-Type': res.headers.get('Content-Type') || 'application/json' },
  })

  if (res.ok) {
    try {
      const data = JSON.parse(text) as CompanyAuthResponse
      if (data.company_user_id && data.token) {
        setCompanySessionCookies(
          response,
          String(data.company_user_id),
          data.token,
          data.refresh_token,
        )
      }
    } catch {
      // 想定外のレスポンス形式でもBackendの応答はそのまま返す（Cookieのみ設定しない）
    }
  }

  return response
}
