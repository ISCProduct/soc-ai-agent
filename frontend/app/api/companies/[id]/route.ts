import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params
    
    const response = await fetch(`${BACKEND_URL}/api/companies/${id}`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('API proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
