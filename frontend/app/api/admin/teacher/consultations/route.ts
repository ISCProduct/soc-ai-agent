import { NextRequest } from 'next/server'
import { proxyAdminBackend, jsonFromProxyResult, proxyErrorResponse, adminProxyHeaders } from '@/lib/admin/backend-proxy'

export const dynamic = 'force-dynamic'

// 先生の相談の受信一覧（#2028）。担当校スコープは Backend が検証する
export async function GET(request: NextRequest) {
  const schoolId = request.nextUrl.searchParams.get('school_id')
  const qs = schoolId && /^\d+$/.test(schoolId) ? `?school_id=${schoolId}` : ''
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
