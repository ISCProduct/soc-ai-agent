import { NextRequest, NextResponse } from 'next/server'
import { extractUserAuthHeaders } from '@/lib/api-proxy'
import { respondWithStudentSession } from '@/lib/auth/issue-session'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

async function handleProfile(request: NextRequest) {
  try {
    const body = await request.json()
    const authHeaders = extractUserAuthHeaders(request)

    // バックエンドのルートは PUT と POST の両方に対応（バージョンによって異なる）
    // まず POST で試みる（旧バージョン互換）
    const response = await fetch(`${BACKEND_URL}/api/auth/profile`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...authHeaders,
      },
      body: JSON.stringify(body),
    })

    // POST が 405 なら PUT で再試行（新バージョン対応）
    if (response.status === 405) {
      const retryResponse = await fetch(`${BACKEND_URL}/api/auth/profile`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          ...authHeaders,
        },
        body: JSON.stringify(body),
      })
      const retryText = await retryResponse.text()
      if (!retryResponse.ok) {
        return NextResponse.json(
          { error: retryText || 'Failed to update profile' },
          { status: retryResponse.status }
        )
      }
      let data
      try { data = JSON.parse(retryText) } catch { data = { message: retryText } }
      return respondWithStudentSession(data, 200)
    }

    const text = await response.text()
    if (!response.ok) {
      return NextResponse.json({ error: text || 'Failed to update profile' }, { status: response.status })
    }

    let data
    try { data = JSON.parse(text) } catch { data = { message: text } }
    // 更新時に Backend が再発行したトークンは Cookie に入れ、画面へは返さない (SOCAIAGENT-683)
    return respondWithStudentSession(data, 200)
  } catch (error) {
    console.error('[auth/profile] error:', error)
    return NextResponse.json({ error: 'Internal server error' }, { status: 500 })
  }
}

export const POST = handleProfile
export const PUT = handleProfile
