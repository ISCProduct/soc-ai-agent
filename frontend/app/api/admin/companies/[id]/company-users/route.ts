import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

function adminHeaders(request: NextRequest): Record<string, string> {
  return {
    'Content-Type': 'application/json',
    'X-Admin-Email': request.headers.get('x-admin-email') || '',
    'X-Admin-Token': request.headers.get('x-admin-token') || '',
  }
}

async function proxyJson(response: Response) {
  const raw = await response.text()
  let data: Record<string, unknown> = {}
  if (raw) {
    try {
      data = JSON.parse(raw) as Record<string, unknown>
    } catch {
      data = response.ok ? { message: raw } : { error: raw }
    }
  }
  return NextResponse.json(data, { status: response.status })
}

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params
  const response = await fetch(`${BACKEND_URL}/api/admin/companies/${id}/company-users`, {
    headers: adminHeaders(request),
  })
  return proxyJson(response)
}

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params
  const body = await request.text()
  const response = await fetch(`${BACKEND_URL}/api/admin/companies/${id}/company-users`, {
    method: 'POST',
    headers: adminHeaders(request),
    body,
  })
  return proxyJson(response)
}
