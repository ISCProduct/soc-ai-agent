import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// Backend側は署名付きstateパラメータにユーザーIDを埋め込むため、
// このプロキシはCookieを一切扱わずJSON({auth_url})を中継するだけでよい。
export async function GET(request: NextRequest) {
  try {
    const res = await fetch(`${BACKEND_URL}/api/google-calendar/connect`, {
      headers: { ...extractUserAuthHeaders(request), Accept: 'application/json' },
    })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
