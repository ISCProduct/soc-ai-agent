import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function PUT(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  try {
    const { id } = await params
    const body = await request.json()
    const response = await fetch(`${BACKEND_URL}/api/applications/${id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        ...extractUserAuthHeaders(request),
      },
      body: JSON.stringify(body),
    })
    const text = await response.text()
    if (!response.ok) {
      return NextResponse.json({ error: text || 'Failed to update status' }, { status: response.status })
    }
    return NextResponse.json(JSON.parse(text))
  } catch (error) {
    console.error('[applications/[id]] PUT error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}
