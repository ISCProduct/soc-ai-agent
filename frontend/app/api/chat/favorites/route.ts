import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const limit = new URL(request.url).searchParams.get('limit') || '50'
    const response = await fetch(`${BACKEND_URL}/api/chat/favorites?limit=${encodeURIComponent(limit)}`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        ...extractUserAuthHeaders(request),
      },
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('[chat/favorites] GET error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
