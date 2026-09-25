import { NextRequest, NextResponse } from 'next/server'
import { clientIpHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function POST(request: NextRequest) {
  const body = await request.text()
  const res = await fetch(`${BACKEND_URL}/api/company-auth/accept-invite`, {
    method: 'POST',
    // IP単位のレート制限を利用者ごとに効かせる(#1407)
    headers: { 'Content-Type': 'application/json', ...clientIpHeaders(request) },
    body,
  })
  const text = await res.text()
  return new NextResponse(text, {
    status: res.status,
    headers: { 'Content-Type': res.headers.get('Content-Type') || 'application/json' },
  })
}
