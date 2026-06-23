import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

function adminHeaders(request: NextRequest) {
  return {
    'Content-Type': 'application/json',
    'X-Admin-Email': request.headers.get('x-admin-email') || '',
    'X-Admin-Token': request.headers.get('x-admin-token') || '',
  }
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ site_key: string }> }
) {
  const { site_key } = await params
  const response = await fetch(`${BACKEND_URL}/api/admin/scraper-sessions/${site_key}`, {
    method: 'DELETE',
    headers: adminHeaders(request),
  })
  if (response.status === 204) {
    return new NextResponse(null, { status: 204 })
  }
  const raw = await response.text()
  let data: Record<string, unknown> = {}
  if (raw) {
    try {
      data = JSON.parse(raw)
    } catch {
      data = response.ok ? { message: raw } : { error: raw }
    }
  }
  return NextResponse.json(data, { status: response.status })
}
