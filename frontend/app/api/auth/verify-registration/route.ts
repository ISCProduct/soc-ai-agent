import { NextRequest } from 'next/server'
import { extractUserAuthHeaders, buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'
import { SERVER_BACKEND_URL } from '@/lib/session-cookies'

// 企業招待・学生仮登録の確認（#1079）。トークンは body に載せ、URL ログに残さない。
export async function POST(request: NextRequest) {
  try {
    const body = await request.text()
    const response = await fetch(`${SERVER_BACKEND_URL}/api/auth/verify-registration`, {
      method: 'POST',
      headers: {
        'Content-Type': request.headers.get('Content-Type') || 'application/json',
        ...extractUserAuthHeaders(request),
      },
      body,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, '登録確認に失敗しました')
  }
}
