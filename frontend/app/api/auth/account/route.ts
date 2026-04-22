import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function DELETE(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    const userId = searchParams.get('user_id')
    if (!userId) {
      return NextResponse.json({ error: 'user_id is required' }, { status: 400 })
    }

    const userID = request.headers.get('X-User-ID') || ''
    const userToken = request.headers.get('X-User-Token') || ''

    const response = await fetch(`${BACKEND_URL}/api/auth/account?user_id=${userId}`, {
      method: 'DELETE',
      headers: {
        'X-User-ID': userID,
        'X-User-Token': userToken,
      },
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
