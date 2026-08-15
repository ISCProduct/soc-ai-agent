import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET() {
  try {
    const response = await fetch(`${BACKEND_URL}/api/whats-new`, { cache: 'no-store' })
    return buildProxyJsonResponse(response)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
