import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const res = await fetch(`${BACKEND_URL}/api/admin/dashboard/export/csv`, {
    headers: {
      'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '',
    },
  })
  if (!res.ok) {
    return NextResponse.json({ error: 'Export failed' }, { status: res.status })
  }
  const csv = await res.text()
  return new NextResponse(csv, {
    status: 200,
    headers: {
      'Content-Type': 'text/csv; charset=utf-8',
      'Content-Disposition': 'attachment; filename="user_scores.csv"',
    },
  })
}
