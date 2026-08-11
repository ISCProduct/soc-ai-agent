import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  try {
    const body = await request.text()
    const response = await fetch(`${BACKEND_URL}/api/es/rewrite`, {
      method: 'POST',
      headers: body ? { 'Content-Type': 'application/json' } : {},
      body: body || undefined,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('ES rewrite proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
