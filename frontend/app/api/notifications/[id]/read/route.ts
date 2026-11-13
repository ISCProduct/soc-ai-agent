import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 自分の通知1件を既読にする（#2033）
export async function POST(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  try {
    const { id } = await params
    if (!/^\d+$/.test(id)) {
      return NextResponse.json({ error: 'id が不正です' }, { status: 400 })
    }
    const res = await fetch(`${BACKEND_URL}/api/notifications/${id}/read`, {
      method: 'POST',
      headers: { ...extractUserAuthHeaders(request) },
    })
    return new NextResponse(null, { status: res.status })
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
