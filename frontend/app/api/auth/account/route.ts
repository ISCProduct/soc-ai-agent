import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function DELETE(request: NextRequest) {
  try {
    // 退会対象は X-User-Token から決まる。クエリの user_id は Backend 側で
    // 読んでおらず、URL とアクセスログに user_id を残すだけだったので送らない。
    const response = await fetch(`${BACKEND_URL}/api/auth/account`, {
      method: 'DELETE',
      headers: extractUserAuthHeaders(request),
    })

    const text = await response.text()
    if (!response.ok) {
      return NextResponse.json({ error: text || 'Failed to delete account' }, { status: response.status })
    }

    let data
    try { data = JSON.parse(text) } catch { data = { message: text } }
    return NextResponse.json(data)
  } catch (error) {
    console.error('[auth/account] DELETE error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}
