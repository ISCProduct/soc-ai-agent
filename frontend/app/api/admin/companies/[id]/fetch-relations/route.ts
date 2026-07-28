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
  const qs = force === 'true' ? '?force=true' : ''
  try {
    const result = await proxyAdminBackend('POST', `/api/admin/companies/${id}/fetch-relations${qs}`, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 180_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
