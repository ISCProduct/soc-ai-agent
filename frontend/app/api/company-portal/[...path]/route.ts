import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

/**
 * 企業ポータルAPIのプロキシ (#1094)。
 * company_id は Backend が JWT から解決するため、ここでは中継のみを行い、
 * クライアントから company_id を受け取って付け替えることはしない。
 */
async function proxy(request: NextRequest, path: string[]) {
  const search = request.nextUrl.search
  const target = `${BACKEND_URL}/api/company-portal/${path.join('/')}${search}`

  const headers: Record<string, string> = {
    'X-Company-User-Token': request.headers.get('x-company-user-token') || '',
  }
  const contentType = request.headers.get('content-type')
  if (contentType) headers['Content-Type'] = contentType

  const hasBody = request.method !== 'GET' && request.method !== 'DELETE'
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
}

type RouteContext = { params: Promise<{ path: string[] }> }

export async function GET(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}

export async function POST(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}
