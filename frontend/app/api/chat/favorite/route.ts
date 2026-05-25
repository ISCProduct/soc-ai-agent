import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()

    const response = await fetch(`${BACKEND_URL}/api/chat/favorite`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...extractUserAuthHeaders(request) },
      body: JSON.stringify(body),
    })

    const text = await response.text()

    if (!response.ok) {
      return NextResponse.json(
        { error: text || 'Failed to toggle favorite' },
        { status: response.status }
      )
    }

    let data
    try {
      data = JSON.parse(text)
    } catch {
      data = { ok: true }
    }

    return NextResponse.json(data)
  } catch (error) {
    console.error('[chat/favorite] POST error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}
