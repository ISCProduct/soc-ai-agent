import { NextRequest } from 'next/server'
import {
  adminProxyHeaders,
  jsonFromProxyResult,
  proxyAdminBackend,
  proxyErrorResponse,
} from '@/lib/admin-backend-proxy'

export const dynamic = 'force-dynamic'
export const maxDuration = 300

/**
 * 主3種（基本・技術・ビジネス関係）を1リクエストで取得する BFF。
 * Backend: POST /api/admin/companies/:id/fetch-primary
 */
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params
  const force = request.nextUrl.searchParams.get('force')
  const qs = force === 'true' ? '?force=true' : ''
  try {
    const result = await proxyAdminBackend('POST', `/api/admin/companies/${id}/fetch-primary${qs}`, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 300_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
