import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 自分の未読をすべて既読にする（#2033）
export async function POST(request: NextRequest) {
  try {
    const res = await fetch(`${BACKEND_URL}/api/notifications/read-all`, {
      method: 'POST',
      headers: { ...extractUserAuthHeaders(request) },
    })
    return new NextResponse(null, { status: res.status })
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'Internal server error')
  }
}
