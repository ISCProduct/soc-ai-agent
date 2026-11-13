import { NextRequest } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 未読の件数（ヘッダのベルのバッジ用、#2033）
export async function GET(request: NextRequest) {
  try {
    const res = await fetch(`${BACKEND_URL}/api/notifications/unread-count`, {
      headers: { ...extractUserAuthHeaders(request) },
    })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
