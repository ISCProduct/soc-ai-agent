import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const userId = searchParams.get('user_id')
  const sessionId = searchParams.get('session_id')

  if (!userId || !sessionId) {
    return NextResponse.json({ error: 'user_id and session_id are required' }, { status: 400 })
  }

  try {
    const res = await fetch(
      `${BACKEND_URL}/api/user/profile?user_id=${encodeURIComponent(userId)}&session_id=${encodeURIComponent(sessionId)}`,
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
