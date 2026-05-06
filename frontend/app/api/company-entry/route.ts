import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  try {
    const body = await request.text()
    const response = await fetch(`${BACKEND_URL}/api/company-entry`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('Company entry proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
