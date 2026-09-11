import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()
    const response = await fetch(`${BACKEND_URL}/api/applications`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...extractUserAuthHeaders(request),
      },
      body: JSON.stringify(body),
    })
    const text = await response.text()
    if (!response.ok) {
      return NextResponse.json({ error: text || 'Failed to apply' }, { status: response.status })
    }
    return NextResponse.json(JSON.parse(text), { status: 201 })
  } catch (error) {
    console.error('[applications] POST error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}

export async function GET(request: NextRequest) {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    if (!authHeaders['X-User-Token']) {
      return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
    }
    const response = await fetch(`${BACKEND_URL}/api/applications`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        ...authHeaders,
      },
      cache: 'no-store',
    })
    const text = await response.text()
    if (!response.ok) {
      return NextResponse.json({ error: text }, { status: response.status })
    }
    return NextResponse.json(JSON.parse(text))
  } catch (error) {
    console.error('[applications] GET error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}
