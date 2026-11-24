import { NextRequest, NextResponse } from 'next/server'
import { clientIpHeaders } from '@/lib/api-proxy'
import { rejectCrossSiteSessionPost } from '@/lib/auth/session-request'
import { respondWithCompanySession } from '@/lib/auth/issue-session'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function POST(request: NextRequest) {
  const rejected = rejectCrossSiteSessionPost(request, 'company-login')
  if (rejected) return rejected
  const body = await request.text()
  const res = await fetch(`${BACKEND_URL}/api/company-auth/login`, {
    method: 'POST',
    // ログイン試行のIP単位制限(20回/分)を利用者ごとに効かせる(#1407)
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
