import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    const sessionId = searchParams.get('session_id')

    if (!sessionId) {
      return NextResponse.json(
        { error: 'session_id is required' },
        { status: 400 }
      )
    }

    const authHeaders = extractUserAuthHeaders(request)
    const userId = authHeaders['X-User-ID']
    const params = new URLSearchParams({ session_id: sessionId })
    if (userId) params.set('user_id', userId)
    const response = await fetch(
      `${BACKEND_URL}/api/chat/scores?${params}`,
      {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
          ...authHeaders,
        },
      }
    )

    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('API proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
