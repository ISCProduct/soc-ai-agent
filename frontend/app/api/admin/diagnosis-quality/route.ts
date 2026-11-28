import { NextRequest, NextResponse } from 'next/server'
import { adminProxyHeaders } from '@/lib/admin/backend-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const query = request.nextUrl.searchParams.toString()
  const response = await fetch(`${BACKEND_URL}/api/admin/diagnosis-quality${query ? `?${query}` : ''}`, {
    headers: adminProxyHeaders(request.headers),
  })
  const raw = await response.text()
  let data: unknown = {}
  if (raw) {
    try {
      data = JSON.parse(raw)
    } catch {
      data = response.ok ? { message: raw } : { error: raw }
    }
  }
  return NextResponse.json(data, { status: response.status })
}
