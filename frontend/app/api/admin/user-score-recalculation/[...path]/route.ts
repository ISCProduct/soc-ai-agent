import { NextRequest, NextResponse } from 'next/server'
import { adminProxyHeaders } from '@/lib/admin/backend-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

function adminHeaders(request: NextRequest) {
  return {
    ...adminProxyHeaders(request.headers),
    'X-Admin-Email': request.headers.get('x-admin-email') || '',
    'X-Admin-Token': request.headers.get('x-admin-token') || '',
  }
}

async function proxy(request: NextRequest, path: string[]) {
  const suffix = path.map(encodeURIComponent).join('/')
  const body = request.method === 'POST' ? await request.text() : undefined
  // クエリ（ドライランの limit、ロールバックの after_id）も Backend へ渡す。落とすと既定値のままになる
  const qs = request.nextUrl.searchParams.toString()
  const response = await fetch(`${BACKEND_URL}/api/admin/user-score-recalculation/${suffix}${qs ? `?${qs}` : ''}`, {
    method: request.method,
    headers: adminHeaders(request),
    body: body || undefined,
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

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  return proxy(request, (await params).path)
}

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  return proxy(request, (await params).path)
}
