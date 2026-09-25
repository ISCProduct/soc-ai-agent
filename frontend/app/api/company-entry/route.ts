import { NextRequest } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, clientIpHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  try {
    const body = await request.text()
    const response = await fetch(`${BACKEND_URL}/api/company-entry`, {
      method: 'POST',
      // ゲスト投稿のIP単位制限(5回/時)と監査ログの送信元IPを利用者ごとにする(#1407)
      headers: { 'Content-Type': 'application/json', ...clientIpHeaders(request) },
      body,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('Company entry proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
