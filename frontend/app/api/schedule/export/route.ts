import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const authHeaders = extractUserAuthHeaders(request)
  if (!authHeaders['X-User-Token']) {
    return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
  }

  const res = await fetch(`${BACKEND_URL}/api/schedule/export/ics`, { headers: authHeaders })
  if (!res.ok) {
    return NextResponse.json({ error: 'Export failed' }, { status: res.status })
  }
  const ics = await res.text()
  return new NextResponse(ics, {
    status: 200,
    headers: {
      'Content-Type': 'text/calendar; charset=utf-8',
      'Content-Disposition': 'attachment; filename="schedule.ics"',
    },
  })
}
