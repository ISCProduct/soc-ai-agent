import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    // 旧バックエンド(ECR)互換の user_id クエリは外した。デプロイ経路は ECS に
    // 一本化済みで、現行 Backend は user_id クエリを読んでいない。
    const response = await fetch(`${BACKEND_URL}/api/chat/sessions`, {
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
