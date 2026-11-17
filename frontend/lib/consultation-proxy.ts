import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

/**
 * 学生の相談 API（#2028）を、同一オリジンの /api/user/consultations から Backend へ中継する。
 * 学生は Backend が X-User-Token から決める。ブラウザから Backend を直接呼ばない（ほかの学生向け API と同じ）。
 */
export async function proxyStudentConsultation(request: NextRequest, path: string, method: 'GET' | 'POST'): Promise<NextResponse> {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    if (!authHeaders['X-User-Token']) {
      return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
    }
    const init: RequestInit = { method, headers: authHeaders, cache: 'no-store' }
    if (method === 'POST') {
      init.headers = { ...authHeaders, 'Content-Type': 'application/json' }
      init.body = await request.text()
    }
    const res = await fetch(`${BACKEND_URL}/api/user/consultations${path}`, init)
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}

/** パスの id が正の整数か。 */
export function isValidId(id: string): boolean {
  return /^[1-9]\d*$/.test(id)
}
