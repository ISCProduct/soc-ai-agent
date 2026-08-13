import { NextRequest, NextResponse } from 'next/server'
import { parseProxyResponse, type ParsedProxyResponse, type ProxyResponseData } from '@/lib/proxy-response'
import { looksLikeHtml, userFacingApiMessage } from '@/lib/user-facing-error'

export interface ProxyErrorBody {
  error: string
  status: number
  detail?: string
}

function getString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined
  }
  const trimmed = value.trim()
  return trimmed.length > 0 ? trimmed : undefined
}

function isProxyErrorObject(data: ParsedProxyResponse): data is ProxyResponseData {
  return !!data && typeof data === 'object' && !Array.isArray(data)
}

function getDetailText(data: ParsedProxyResponse, raw: string): string | undefined {
  const detail = isProxyErrorObject(data)
    ? getString(data.detail) ??
      getString(data.details) ??
      getString(data.message) ??
      getString(raw)
    : getString(raw)
  return detail
}

export function extractUserAuthHeaders(request: NextRequest): Record<string, string> {
  const headers: Record<string, string> = {}
  const xUserId = request.headers.get('X-User-ID')
  const xUserToken = request.headers.get('X-User-Token')
  const xTenantSlug = request.headers.get('X-Tenant-Slug')
  if (xUserId) headers['X-User-ID'] = xUserId
  if (xUserToken) headers['X-User-Token'] = xUserToken
  if (xTenantSlug) headers['X-Tenant-Slug'] = xTenantSlug
  return headers
}

export async function buildProxyJsonResponse(response: Response): Promise<NextResponse> {
  const raw = await response.text()
  const data = parseProxyResponse(raw, response.ok)

  if (response.ok) {
    return NextResponse.json(data, { status: response.status })
  }

  const rawForUser = looksLikeHtml(raw) ? '' : raw
  const error = userFacingApiMessage(response.status, raw)
  const detail = looksLikeHtml(raw) ? undefined : getDetailText(data, rawForUser)
  const body: ProxyErrorBody = {
    error,
    status: response.status,
    ...(detail && detail !== error && !looksLikeHtml(detail) ? { detail } : {}),
  }
  return NextResponse.json(body, { status: response.status })
}

export function buildProxyNetworkErrorResponse(error: unknown, message: string): NextResponse {
  const detail = error instanceof Error ? error.message : String(error)
  const body: ProxyErrorBody = {
    error: message,
    status: 500,
    ...(detail ? { detail } : {}),
  }
  return NextResponse.json(body, { status: 500 })
}
