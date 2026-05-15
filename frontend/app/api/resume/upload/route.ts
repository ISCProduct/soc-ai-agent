import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

function userHeaders(request: NextRequest): Record<string, string> {
  return {
    'X-User-ID': request.headers.get('X-User-ID') || '',
    'X-User-Token': request.headers.get('X-User-Token') || '',
  }
}

export async function POST(request: NextRequest) {
  try {
    const formData = await request.formData()
    const response = await fetch(`${BACKEND_URL}/api/resume/upload`, {
      method: 'POST',
      headers: userHeaders(request),
      body: formData,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('Resume upload proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
