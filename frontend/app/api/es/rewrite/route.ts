import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  try {
    const body = await request.text()
    // 未ログイン利用の既存動作は維持しつつ、認証ヘッダーがあれば転送する(#1015)
    const headers: Record<string, string> = extractUserAuthHeaders(request)
    if (body) {
      headers['Content-Type'] = 'application/json'
    }
    const response = await fetch(`${BACKEND_URL}/api/es/rewrite`, {
      method: 'POST',
      headers,
      body: body || undefined,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('ES rewrite proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
