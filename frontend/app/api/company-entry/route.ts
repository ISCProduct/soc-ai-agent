import { NextRequest, NextResponse } from 'next/server'
import { parseProxyResponse } from '@/lib/proxy-response'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  const body = await request.text()
  const response = await fetch(`${BACKEND_URL}/api/company-entry`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  })
  const raw = await response.text()
  const data = parseProxyResponse(raw, response.ok)
  return NextResponse.json(data, { status: response.status })
}
