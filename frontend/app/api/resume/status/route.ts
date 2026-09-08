import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const authHeaders = extractUserAuthHeaders(request)
  // 未認証ならバックエンドへ往復せず打ち切る（whats-new プロキシと同じ）。
  // ホーム画面表示のたびに叩くため、無駄な往復を増やさない。
  if (!authHeaders['X-User-Token']) {
    return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
  }
  try {
    const response = await fetch(`${BACKEND_URL}/api/resume/status`, {
      cache: 'no-store',
      headers: authHeaders,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('resume status proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
