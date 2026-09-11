import { NextRequest } from 'next/server'
import {
  proxyAdminBackend,
  jsonFromProxyResult,
  proxyErrorResponse,
  adminProxyHeaders,
} from '@/lib/admin-backend-proxy'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const query = request.nextUrl.searchParams.toString()
  const path = `/api/admin/companies${query ? `?${query}` : ''}`
  try {
    const result = await proxyAdminBackend('GET', path, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}

export async function POST(request: NextRequest) {
  const body = await request.text()
  try {
    const result = await proxyAdminBackend('POST', '/api/admin/companies', {
      headers: adminProxyHeaders(request.headers),
      body,
      timeoutMs: 30_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
