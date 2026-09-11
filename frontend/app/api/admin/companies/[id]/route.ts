import { NextRequest } from 'next/server'
import {
  proxyAdminBackend,
  jsonFromProxyResult,
  proxyErrorResponse,
  adminProxyHeaders,
} from '@/lib/admin-backend-proxy'

export const dynamic = 'force-dynamic'

function parseCompanyId(id: string): number | null {
  const n = Number(id)
  return Number.isInteger(n) && n > 0 ? n : null
}

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params
  if (!parseCompanyId(id)) {
    return Response.json({ error: 'invalid company id' }, { status: 400 })
  }
  try {
    const result = await proxyAdminBackend('GET', `/api/admin/companies/${id}`, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params
  if (!parseCompanyId(id)) {
    return Response.json({ error: 'invalid company id' }, { status: 400 })
  }
  const body = await request.text()
  try {
    const result = await proxyAdminBackend('PUT', `/api/admin/companies/${id}`, {
      headers: adminProxyHeaders(request.headers),
      body,
      timeoutMs: 30_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
