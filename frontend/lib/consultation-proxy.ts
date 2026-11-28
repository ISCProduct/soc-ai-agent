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
    if (PASS_THROUGH_STATUSES.has(res.status)) {
      return passThroughUserError(res)
    }
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}

// Backend が学生向けに書いた理由（学校が未登録・未回答の質問が多い・入力の誤り・見つからない）を
// そのまま見せる応答の status。共通のプロキシは 4xx を一般的な文言に置き換えるので、学生が
// 何をすればよいか分からなくなる。401/403/5xx は内部の情報を含みうるので、共通の処理に任せる。
const PASS_THROUGH_STATUSES = new Set([400, 404, 429])

/** Backend の {"error": "..."} を、同じ status の {"error": "..."} として返す（読めなければ一般的な文言）。 */
async function passThroughUserError(res: Response): Promise<NextResponse> {
  const data = (await res.json().catch(() => ({}))) as { error?: unknown }
  const error = typeof data.error === 'string' && data.error.trim() ? data.error : '処理に失敗しました。しばらくしてから再試行してください。'
  return NextResponse.json({ error, status: res.status }, { status: res.status })
}

/** パスの id が正の整数か。 */
export function isValidId(id: string): boolean {
  return /^[1-9]\d*$/.test(id)
}
