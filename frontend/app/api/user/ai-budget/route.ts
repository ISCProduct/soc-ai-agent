import { NextRequest } from 'next/server'
import {
  buildProxyJsonResponse,
  buildProxyNetworkErrorResponse,
  extractUserAuthHeaders,
} from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const headers = extractUserAuthHeaders(request)
  if (!headers['X-User-Token']) {
    return Response.json({ error: '認証が必要です' }, { status: 401 })
  }

  try {
    const response = await fetch(`${BACKEND_URL}/api/user/ai-budget`, {
      headers,
      cache: 'no-store',
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('AI budget proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
