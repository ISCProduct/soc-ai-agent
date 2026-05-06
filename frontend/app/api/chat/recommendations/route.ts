import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    const userId = searchParams.get('user_id')
    const sessionId = searchParams.get('session_id')
    const limit = searchParams.get('limit') || '5'
    
    if (!userId || !sessionId) {
      return NextResponse.json(
        { error: 'user_id and session_id are required' },
        { status: 400 }
      )
    }

    const response = await fetch(
      `${BACKEND_URL}/api/chat/recommendations?user_id=${userId}&session_id=${sessionId}&limit=${limit}`,
      {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }
    )

    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('API proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
