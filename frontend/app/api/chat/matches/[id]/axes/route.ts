import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 推薦1件の、軸別のマッチの内訳（SOCAIAGENT-611）。本人の推薦かどうかは Backend が X-User-Token で確かめる
export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  if (!/^[1-9]\d*$/.test(id)) {
    return NextResponse.json({ error: 'id が不正です' }, { status: 400 })
  }
  try {
    const authHeaders = extractUserAuthHeaders(request)
    if (!authHeaders['X-User-Token']) {
      return NextResponse.json({ error: '認証が必要です' }, { status: 401 })
    }
    const res = await fetch(`${BACKEND_URL}/api/chat/matches/${id}/axes`, { headers: authHeaders, cache: 'no-store' })
    return buildProxyJsonResponse(res)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
