import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 企業検索の絞り込みの選択肢（業界・都道府県・職種。SOCAIAGENT-612）
export async function GET() {
  try {
    const res = await fetch(`${BACKEND_URL}/api/companies/search-options`, { cache: 'no-store' })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
