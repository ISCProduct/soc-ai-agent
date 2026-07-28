import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params
  const body = await request.text()
  const response = await fetch(`${BACKEND_URL}/api/admin/companies/${id}/confirm-relations`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '',
    },
    body,
    signal: AbortSignal.timeout(60_000),
  })
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
