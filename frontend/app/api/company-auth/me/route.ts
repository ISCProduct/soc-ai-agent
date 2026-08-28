import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function GET(request: NextRequest) {
  const res = await fetch(`${BACKEND_URL}/api/company-auth/me`, {
    headers: {
      'X-Company-User-ID': request.headers.get('x-company-user-id') || '',
      'X-Company-User-Token': request.headers.get('x-company-user-token') || '',
    },
  })
  const text = await res.text()
  return new NextResponse(text, {
    status: res.status,
    headers: { 'Content-Type': res.headers.get('Content-Type') || 'application/json' },
  })
}
