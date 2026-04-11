import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const months = searchParams.get('months') || '12'
  const res = await fetch(`${BACKEND_URL}/api/admin/costs/monthly?months=${months}`, {
    headers: {
      'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '',
    },
  })
  const data = await res.json()
  return NextResponse.json(data, { status: res.status })
}
