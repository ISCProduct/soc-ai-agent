import { NextRequest } from 'next/server'
import {
  proxyAdminBackend,
  jsonFromProxyResult,
  proxyErrorResponse,
  adminProxyHeaders,
} from '@/lib/admin/backend-proxy'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const body = await request.text()
  try {
    const result = await proxyAdminBackend('POST', `/api/admin/teacher/students/${id}/guidances`, {
      headers: adminProxyHeaders(request.headers),
      body,
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
