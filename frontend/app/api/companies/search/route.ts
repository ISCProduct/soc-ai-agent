import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 学生向けの企業検索（SOCAIAGENT-612）。条件の検証は Backend が行う
export async function GET(request: NextRequest) {
  const qs = new URL(request.url).searchParams
  const params = new URLSearchParams()
  for (const key of ['name', 'industry', 'prefecture', 'job_category_id', 'limit', 'offset']) {
    const v = qs.get(key)
    if (v) params.set(key, v)
  }
  try {
    const res = await fetch(`${BACKEND_URL}/api/companies/search${params.size ? `?${params}` : ''}`, { cache: 'no-store' })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
