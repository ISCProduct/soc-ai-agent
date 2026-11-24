import { NextRequest, NextResponse } from 'next/server'
import { clientIpHeaders } from '@/lib/api-proxy'
import { respondWithCompanySession } from '@/lib/auth/issue-session'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

// パスワード再設定を実行する（認証不要）。#1196
//
// ログイン・登録・招待の受け入れと同じく、成功したらここで httpOnly Cookie を張る（SOCAIAGENT-683）。
export async function POST(request: NextRequest) {
  const body = await request.text()
  const res = await fetch(`${BACKEND_URL}/api/company-auth/reset-password`, {
    method: 'POST',
    // IP単位のレート制限を利用者ごとに効かせる(#1407)
    headers: { 'Content-Type': 'application/json', ...clientIpHeaders(request) },
    body,
  })
  // 成功したらここで httpOnly Cookie を張り、トークンを画面へ返さない（SOCAIAGENT-683）
  if (res.ok && (res.headers.get('Content-Type') ?? '').includes('application/json')) {
    const data: unknown = await res.json().catch(() => null)
    if (data && typeof data === 'object') return respondWithCompanySession(data as Record<string, unknown>, res.status)
  }
  const text = await res.text()
  return new NextResponse(text, {
    status: res.status,
    headers: { 'Content-Type': res.headers.get('Content-Type') || 'application/json' },
  })
}
