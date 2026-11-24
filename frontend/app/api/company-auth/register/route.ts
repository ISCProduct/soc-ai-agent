import { NextRequest, NextResponse } from 'next/server'
import { clientIpHeaders } from '@/lib/api-proxy'
import { respondWithCompanySession } from '@/lib/auth/issue-session'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function POST(request: NextRequest) {
  const body = await request.text()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...clientIpHeaders(request),
  }
  // middleware が採番したリクエストIDを Backend まで引き継ぐ（#1188）
  const requestId = request.headers.get('X-Request-ID')
  if (requestId) headers['X-Request-ID'] = requestId
  try {
    const res = await fetch(`${BACKEND_URL}/api/company-auth/register`, {
      method: 'POST',
      headers,
      body,
    })
    // 成功したらここで httpOnly Cookie を張り、トークンを画面へ返さない（SOCAIAGENT-683）
    if (res.ok && (res.headers.get('Content-Type') ?? '').includes('application/json')) {
      const data: unknown = await res.json().catch(() => null)
      if (data && typeof data === 'object') return respondWithCompanySession(data as Record<string, unknown>, res.status)
    }
    const text = await res.text()
    return new NextResponse(text || null, {
      status: res.status,
      headers: { 'Content-Type': res.headers.get('Content-Type') || 'application/json' },
    })
  } catch (err) {
    const detail = err instanceof Error ? err.message : 'unknown'
    return NextResponse.json(
      {
        error: 'Backend に接続できませんでした。Backend が起動しているか確認してください。',
        code: 'SERVICE_UNAVAILABLE',
        detail,
      },
      { status: 503 },
    )
  }
}
