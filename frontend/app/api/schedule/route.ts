import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    const res = await fetch(`${BACKEND_URL}/api/schedule`, {
      headers: { ...authHeaders },
    })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}

export async function POST(request: NextRequest) {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    const body = await request.text()
    const res = await fetch(`${BACKEND_URL}/api/schedule`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders },
      body,
    })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
