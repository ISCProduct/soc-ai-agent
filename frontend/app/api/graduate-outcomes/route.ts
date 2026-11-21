import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 卒業生の進路実績（#2032）。学生と学校は Backend が X-User-Token から決める。
export async function GET(request: NextRequest) {
  try {
    const authHeaders = extractUserAuthHeaders(request)
    if (!authHeaders['X-User-Token']) {
      return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
    }
    const qs = new URL(request.url).searchParams
    const params = new URLSearchParams()
    for (const key of ['year', 'industry']) {
      const v = qs.get(key)
      if (v) params.set(key, v)
    }
    const res = await fetch(`${BACKEND_URL}/api/graduate-outcomes${params.size ? `?${params}` : ''}`, { headers: authHeaders })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
