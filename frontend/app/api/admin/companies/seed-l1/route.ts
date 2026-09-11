import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  const contentType = request.headers.get('content-type') || ''
  const headers: HeadersInit = {
    'X-Admin-Email': request.headers.get('x-admin-email') || '',
    'X-Admin-Token': request.headers.get('x-admin-token') || '',
  }
  let body: BodyInit | undefined
  if (contentType.includes('multipart/form-data')) {
    body = await request.formData()
  } else {
    headers['Content-Type'] = contentType || 'text/csv'
    body = await request.text()
  }
  const qs = request.nextUrl.searchParams.toString()
  const response = await fetch(
    `${BACKEND_URL}/api/admin/companies/seed-l1${qs ? `?${qs}` : ''}`,
    { method: 'POST', headers, body },
  )
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
