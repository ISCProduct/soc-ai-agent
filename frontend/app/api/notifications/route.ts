import { NextRequest } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// アプリ内通知の一覧（#2033）。宛先は X-User-Token から Backend が決める。
export async function GET(request: NextRequest) {
  try {
    const qs = new URL(request.url).searchParams
    const params = new URLSearchParams()
    for (const key of ['limit', 'before_id']) {
      const v = qs.get(key)
      if (v) params.set(key, v)
    }
    const res = await fetch(`${BACKEND_URL}/api/notifications${params.size ? `?${params}` : ''}`, {
      headers: { ...extractUserAuthHeaders(request) },
    })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
