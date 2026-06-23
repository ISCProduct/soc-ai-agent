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

async function parseResponse(response: Response) {
  const raw = await response.text()
  if (!raw) return {}
  try {
    return JSON.parse(raw)
  } catch {
    return response.ok ? { message: raw } : { error: raw }
  }
}

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ company_id: string }> }
) {
  const { company_id } = await params
  const response = await fetch(`${BACKEND_URL}/api/admin/profile-recalculation/${company_id}/rollback`, {
    method: 'POST',
    headers: adminHeaders(request),
  })
  return NextResponse.json(await parseResponse(response), { status: response.status })
}
