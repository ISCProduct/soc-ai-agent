import { NextRequest, NextResponse } from 'next/server'
import { clientIpHeaders } from '@/lib/api-proxy'

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
