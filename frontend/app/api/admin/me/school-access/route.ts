import { NextRequest } from 'next/server'
import {
  proxyAdminBackend,
  jsonFromProxyResult,
  proxyErrorResponse,
  adminProxyHeaders,
} from '@/lib/admin-backend-proxy'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const result = await proxyAdminBackend('GET', '/api/admin/me/school-access', {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
