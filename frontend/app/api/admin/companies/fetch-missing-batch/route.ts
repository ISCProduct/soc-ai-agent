import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  const body = await request.text()
  const response = await fetch(`${BACKEND_URL}/api/admin/companies/fetch-missing-batch`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '',
    },
    body: body || '{}',
    // 最大50社 × 複数取得のため長め
    signal: AbortSignal.timeout(600_000),
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
