import { NextRequest } from 'next/server'
import { proxyAdminBackend, jsonFromProxyResult, proxyErrorResponse, adminProxyHeaders } from '@/lib/admin/backend-proxy'

export const dynamic = 'force-dynamic'

// 先生の相談の受信一覧（#2028）。担当校スコープは Backend が検証する。続きは before_id で取る
export async function GET(request: NextRequest) {
  const params = new URLSearchParams()
  for (const key of ['school_id', 'before_id', 'limit']) {
    const v = request.nextUrl.searchParams.get(key)
    if (v && /^\d+$/.test(v)) params.set(key, v)
  }
  const qs = params.size ? `?${params}` : ''
  try {
    const result = await proxyAdminBackend('GET', `/api/admin/teacher/consultations${qs}`, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
