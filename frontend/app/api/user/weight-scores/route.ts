import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const sessionId = searchParams.get('session_id')

  if (!sessionId) {
    return NextResponse.json({ error: 'session_id is required' }, { status: 400 })
  }

  try {
    // 認証ヘッダを渡す。これが無いと /api/user/profile は EchoUserAuth で 401 を返し、
    // 下の !res.ok に落ちて常に空配列になる（履歴書の前後比較と面接のスコア表示が
    // 無言で動かなくなっていた）。
    const res = await fetch(
      `${BACKEND_URL}/api/user/profile?session_id=${encodeURIComponent(sessionId)}`,
      { headers: extractUserAuthHeaders(request) },
    )
    if (!res.ok) {
      return NextResponse.json({ weight_scores: [] }, { status: 200 })
    }
    const data = await res.json()
    return NextResponse.json({ weight_scores: data.weight_scores ?? [] })
  } catch {
    return NextResponse.json({ weight_scores: [] }, { status: 200 })
  }
}
