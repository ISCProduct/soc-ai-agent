import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 自己分析ポートフォリオ（#2030）。本人は X-User-Token から Backend が決める（user_id は渡さない）。
export async function GET(request: NextRequest) {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    if (!authHeaders['X-User-Token']) {
      return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
    }
    const sections = new URL(request.url).searchParams.get('sections') ?? ''
    const qs = sections ? `?sections=${encodeURIComponent(sections)}` : ''
    const res = await fetch(`${BACKEND_URL}/api/portfolio${qs}`, { headers: authHeaders, cache: 'no-store' })
    const response = await buildProxyJsonResponse(res)
    response.headers.set('Cache-Control', 'no-store')
    return response
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
