import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const params = new URL(request.url).searchParams
    const query = new URLSearchParams({ limit: params.get('limit') || '50' })
    const offset = params.get('offset')
    if (offset) query.set('offset', offset)
    const response = await fetch(`${BACKEND_URL}/api/chat/favorites?${query.toString()}`, {
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
