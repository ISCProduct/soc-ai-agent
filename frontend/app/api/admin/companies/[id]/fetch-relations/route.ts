import { NextRequest } from 'next/server'
import {
  adminProxyHeaders,
  jsonFromProxyResult,
  proxyAdminBackend,
  proxyErrorResponse,
} from '@/lib/admin-backend-proxy'

export const dynamic = 'force-dynamic'
export const maxDuration = 180

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params
  const force = request.nextUrl.searchParams.get('force')
  const cacheOnly = request.nextUrl.searchParams.get('cache_only')
  const qs = new URLSearchParams()
  if (force === 'true') qs.set('force', 'true')
  if (cacheOnly === 'true') qs.set('cache_only', 'true')
  const query = qs.toString() ? `?${qs.toString()}` : ''
  try {
    const result = await proxyAdminBackend('POST', `/api/admin/companies/${id}/fetch-relations${query}`, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 120_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
