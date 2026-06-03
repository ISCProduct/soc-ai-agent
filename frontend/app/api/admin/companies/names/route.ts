import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const query = request.nextUrl.searchParams.toString()
  const response = await fetch(`${BACKEND_URL}/api/admin/companies/names${query ? `?${query}` : ''}`, {
    headers: {
      'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '',
    },
  })
  const raw = await response.text()
  // backend returns JSON array; try parse, otherwise wrap
  if (!raw) return NextResponse.json([], { status: response.status })
  try {
    const data = JSON.parse(raw)
    return NextResponse.json(data, { status: response.status })
  } catch {
    return NextResponse.json(response.ok ? { message: raw } : { error: raw }, { status: response.status })
  }
}
