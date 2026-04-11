import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const name = request.nextUrl.searchParams.get('name') || ''
  const response = await fetch(
    `${BACKEND_URL}/api/admin/companies/search-gbiz?name=${encodeURIComponent(name)}`,
    {
      headers: { 'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '' },
    },
  )
  const raw = await response.text()
  let data: any = {}
  if (raw) {
    try { data = JSON.parse(raw) } catch { data = { error: raw } }
  }
  return NextResponse.json(data, { status: response.status })
}
