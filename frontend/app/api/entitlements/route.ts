import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    const response = await fetch(`${BACKEND_URL}/api/entitlements`, {
      cache: 'no-store',
      headers: { ...authHeaders },
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
