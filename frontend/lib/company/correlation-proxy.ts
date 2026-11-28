import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

export function validCompanyId(value: string | null): value is string {
  return !!value && /^[1-9]\d*$/.test(value) && Number.isSafeInteger(Number(value))
}
export async function proxyCorrelation(request: NextRequest, companyId: string, compare = false) {
  const otherId = request.nextUrl.searchParams.get('with')
  if (!validCompanyId(companyId) || (compare && !validCompanyId(otherId))) {
    return NextResponse.json({ error: '企業を選び直してください。' }, { status: 400 })
  }
  const suffix = compare ? `/compare?with=${otherId}` : ''
  try {
    const response = await fetch(`${process.env.BACKEND_URL || 'http://app:8080'}/api/companies/${companyId}/correlation-map${suffix}`, {
      headers: extractUserAuthHeaders(request), cache: 'no-store', signal: request.signal,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, '企業の情報を取得できませんでした。もう一度お試しください。')
  }
}
