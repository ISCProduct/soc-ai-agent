import { NextRequest } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const response = await fetch(`${BACKEND_URL}/api/resume/status`, {
      cache: 'no-store',
      headers: extractUserAuthHeaders(request),
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
