import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 応募を受け付けているか（Backend の STUDENT_APPLY_ENABLED）。実行時に Backend から取るので、
// 切り替えに再ビルドは要らない。
export async function GET(request: NextRequest) {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    if (!authHeaders['X-User-Token']) {
      return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
    }
    const response = await fetch(`${BACKEND_URL}/api/applications/availability`, {
      headers: { 'Content-Type': 'application/json', ...authHeaders },
      cache: 'no-store',
    })
    const text = await response.text()
    if (!response.ok) {
      return NextResponse.json({ error: text }, { status: response.status })
    }
    return NextResponse.json(JSON.parse(text))
  } catch (error) {
    console.error('[applications/availability] GET error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}
