import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params
  const response = await fetch(
    `${BACKEND_URL}/api/admin/companies/${id}/interview-questions/generate`,
    {
      method: 'POST',
      headers: {
        'X-Admin-Email': request.headers.get('x-admin-email') || '',
        'X-Admin-Token': request.headers.get('x-admin-token') || '',
      },
      signal: AbortSignal.timeout(90_000),
    },
  )
  const data = await response.json().catch(() => ({}))
  return NextResponse.json(data, { status: response.status })
}
