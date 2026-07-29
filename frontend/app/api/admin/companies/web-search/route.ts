import { NextRequest } from 'next/server'
import {
  adminProxyHeaders,
  jsonFromProxyResult,
  proxyAdminBackend,
  proxyErrorResponse,
} from '@/lib/admin-backend-proxy'

export const dynamic = 'force-dynamic'
export const maxDuration = 180

export async function POST(request: NextRequest) {
  const body = await request.text()
  try {
    const result = await proxyAdminBackend('POST', '/api/admin/companies/web-search', {
      headers: adminProxyHeaders(request.headers),
      body,
      timeoutMs: 180_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
