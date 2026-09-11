import { NextRequest } from 'next/server'
import {
  proxyAdminBackend,
  jsonFromProxyResult,
  proxyErrorResponse,
  adminProxyHeaders,
} from '@/lib/admin-backend-proxy'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  try {
    const result = await proxyAdminBackend('GET', `/api/admin/organizations/${id}`, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}

export async function PUT(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const body = await request.text()
  try {
    const result = await proxyAdminBackend('PUT', `/api/admin/organizations/${id}`, {
      headers: adminProxyHeaders(request.headers),
      body,
      timeoutMs: 30_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
