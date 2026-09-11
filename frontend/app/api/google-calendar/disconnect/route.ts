import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function DELETE(request: NextRequest) {
  try {
    const res = await fetch(`${BACKEND_URL}/api/google-calendar/disconnect`, {
      method: 'DELETE',
      headers: extractUserAuthHeaders(request),
    })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
