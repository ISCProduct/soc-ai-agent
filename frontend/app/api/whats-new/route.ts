import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'
import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const authHeaders = extractUserAuthHeaders(request)
  if (!authHeaders['X-User-Token']) {
    return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
  }
  try {
    const response = await fetch(`${BACKEND_URL}/api/whats-new`, {
      cache: 'no-store',
      headers: authHeaders,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
