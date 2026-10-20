import { NextRequest } from 'next/server'
import {
  adminProxyHeaders,
  jsonFromProxyResult,
  proxyAdminBackend,
  proxyErrorResponse,
} from '@/lib/admin/backend-proxy'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const result = await proxyAdminBackend('GET', '/api/admin/ai-budgets', {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}

export async function PUT(request: NextRequest) {
  try {
    const result = await proxyAdminBackend('PUT', '/api/admin/ai-budgets', {
      headers: adminProxyHeaders(request.headers),
      body: await request.text(),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}

export async function DELETE(request: NextRequest) {
  try {
    const scope = request.nextUrl.searchParams.get('scope')
    const scopeID = request.nextUrl.searchParams.get('scope_id')
    if ((scope !== 'user' && scope !== 'school') || !scopeID || !/^[1-9]\d*$/.test(scopeID)) {
      return Response.json({ error: '設定対象が不正です' }, { status: 400 })
    }
    const result = await proxyAdminBackend(
      'DELETE',
      `/api/admin/ai-budgets/${scope}/${scopeID}`,
      {
        headers: adminProxyHeaders(request.headers),
        timeoutMs: 15_000,
      },
    )
    if (result.status === 204) return new Response(null, { status: 204 })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
