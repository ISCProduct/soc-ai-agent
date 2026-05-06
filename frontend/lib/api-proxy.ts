import { NextResponse } from 'next/server'
import { parseProxyResponse, type ProxyResponseData } from '@/lib/proxy-response'

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

function getErrorText(data: ProxyResponseData): string {
  return getString(data.error) ?? getString(data.message) ?? 'Upstream API error'
}

function getDetailText(data: ProxyResponseData, raw: string): string | undefined {
  const detail =
    getString(data.detail) ??
    getString(data.details) ??
    getString(data.message) ??
    getString(raw)
  return detail
}

export async function buildProxyJsonResponse(response: Response): Promise<NextResponse> {
  const raw = await response.text()
  const data = parseProxyResponse(raw, response.ok)

  if (response.ok) {
    return NextResponse.json(data, { status: response.status })
  }

  const error = getErrorText(data)
  const detail = getDetailText(data, raw)
  const body: ProxyErrorBody = {
    error,
    status: response.status,
    ...(detail && detail !== error ? { detail } : {}),
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
