import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

type RouteContext = { params: Promise<{ path?: string[] }> }

/** 適性検査 API のプロキシ（#1678）。本人の判定は Backend が X-User-Token から行う。 */
async function proxy(request: NextRequest, context: RouteContext) {
  const headers = extractUserAuthHeaders(request)
  if (!headers['X-User-Token']) {
    return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
  }
  const path = (await context.params).path ?? []
  const target = `${BACKEND_URL}/api/assessments${path.length ? `/${path.map(encodeURIComponent).join('/')}` : ''}`
  const hasBody = request.method === 'POST' || request.method === 'PUT'
  if (hasBody) headers['Content-Type'] = 'application/json'
  try {
    const response = await fetch(target, {
      method: request.method,
      headers,
      body: hasBody ? await request.text() : undefined,
      cache: 'no-store',
    })
    // 204 は本文を持てないので JSON 化せずに返す
    if (response.status === 204) return new NextResponse(null, { status: 204 })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('assessments proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}

export const GET = proxy
export const POST = proxy
export const PUT = proxy
