import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    // 旧バックエンド（ECR）との互換性のため user_id クエリパラメータも付与する
    const userId = authHeaders['X-User-ID']
    const url = userId
      ? `${BACKEND_URL}/api/chat/sessions?user_id=${userId}`
      : `${BACKEND_URL}/api/chat/sessions`
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        ...authHeaders,
      },
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('API proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
