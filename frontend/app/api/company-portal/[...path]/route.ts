import { NextRequest, NextResponse } from 'next/server'
import { buildProxyNetworkErrorResponse, clientIpHeaders, encodePathSegments } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

/**
 * 企業ポータルAPIのプロキシ (#1094)。
 * company_id は Backend が JWT から解決するため、ここでは中継のみを行い、
 * クライアントから company_id を受け取って付け替えることはしない。
 * 学生用の認証ヘッダーは引き継がない（企業トークンだけで認可させる）。
 */
async function proxy(request: NextRequest, path: string[]) {
  const encodedPath = encodePathSegments(path)
  if (encodedPath === null) {
    return NextResponse.json({ error: '不正なリクエストです', status: 400 }, { status: 400 })
  }
  const target = `${BACKEND_URL}/api/company-portal/${encodedPath}${request.nextUrl.search}`

  const headers: Record<string, string> = {
    ...clientIpHeaders(request),
    'X-Company-User-Token': request.headers.get('x-company-user-token') || '',
  }
  for (const name of ['Content-Type', 'X-Request-ID', 'X-Tenant-Slug']) {
    const value = request.headers.get(name)
    if (value) headers[name] = value
  }

  const hasBody = request.method !== 'GET' && request.method !== 'DELETE'
  try {
    const res = await fetch(target, {
      method: request.method,
      headers,
      body: hasBody ? await request.text() : undefined,
      cache: 'no-store',
    })

    const text = await res.text()
    return new NextResponse(text || null, {
      status: res.status,
      headers: { 'Content-Type': res.headers.get('Content-Type') || 'application/json' },
    })
  } catch (error) {
    console.error('company-portal proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}

type RouteContext = { params: Promise<{ path: string[] }> }

export async function GET(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}

export async function POST(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}

export async function PATCH(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}
