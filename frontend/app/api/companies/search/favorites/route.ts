import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 検索結果の企業のお気に入りの状態（本人の推薦にある企業だけ。SOCAIAGENT-612）
export async function GET(request: NextRequest) {
  const ids = new URL(request.url).searchParams.get('company_ids') ?? ''
  if (!/^[\d,]*$/.test(ids)) {
    return NextResponse.json({ error: 'company_ids が不正です' }, { status: 400 })
  }
  try {
    const authHeaders = extractUserAuthHeaders(request)
    if (!authHeaders['X-User-Token']) {
      return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
    }
    const res = await fetch(`${BACKEND_URL}/api/companies/search/favorites?company_ids=${ids}`, { headers: authHeaders, cache: 'no-store' })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
