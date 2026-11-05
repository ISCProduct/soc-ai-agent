import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  try {
    const body = await request.text()
    const response = await fetch(`${BACKEND_URL}/api/es/review`, {
      method: 'POST',
      // ゲストAIのIP単位制限を利用者ごとに効かせる(#1407)。
      // あわせて X-User-Token も渡す。Backend は未ログインなら素通りし、
      // ログイン中のときだけ添削結果を履歴として保存する(#1531)。
      headers: body
        ? { 'Content-Type': 'application/json', ...extractUserAuthHeaders(request) }
        : extractUserAuthHeaders(request),
      body: body || undefined,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('ES review proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
