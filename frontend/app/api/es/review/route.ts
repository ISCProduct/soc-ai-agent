import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, clientIpHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  try {
    const body = await request.text()
    const response = await fetch(`${BACKEND_URL}/api/es/review`, {
      method: 'POST',
      // ゲストAIのIP単位制限を利用者ごとに効かせる(#1407)
      headers: body
        ? { 'Content-Type': 'application/json', ...clientIpHeaders(request) }
        : clientIpHeaders(request),
      body: body || undefined,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('ES review proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
