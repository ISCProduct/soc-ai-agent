import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 「あなたの10の傾向」（SOCAIAGENT-611）。軸ごとの内訳と同じく、適性検査を合流させた値。
// 本人のスコアだけを返すことは Backend が X-User-Token で確かめる
export async function GET(request: NextRequest) {
  const sessionId = new URL(request.url).searchParams.get('session_id')
  if (!sessionId) {
    return NextResponse.json({ error: 'session_id is required' }, { status: 400 })
  }
  try {
    const authHeaders = extractUserAuthHeaders(request)
    if (!authHeaders['X-User-Token']) {
      return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
    }
    const params = new URLSearchParams({ session_id: sessionId })
    const res = await fetch(`${BACKEND_URL}/api/chat/axis-scores?${params}`, { headers: authHeaders, cache: 'no-store' })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
