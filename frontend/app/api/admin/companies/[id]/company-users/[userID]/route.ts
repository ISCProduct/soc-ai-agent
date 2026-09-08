import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 親の company-users/route.ts (GET/POST) と同じ転送方式。
// 認証は Backend 側で判定するため、ヘッダーが無ければ空文字のまま転送して 401 をそのまま返す。
function adminHeaders(request: NextRequest): Record<string, string> {
  return {
    'Content-Type': 'application/json',
    'X-Admin-Email': request.headers.get('x-admin-email') || '',
    'X-Admin-Token': request.headers.get('x-admin-token') || '',
  }
}

async function proxyJson(response: Response) {
  const raw = await response.text()
  let data: Record<string, unknown> = {}
  if (raw) {
    try {
      data = JSON.parse(raw) as Record<string, unknown>
    } catch {
      data = response.ok ? { message: raw } : { error: raw }
    }
  }
  return NextResponse.json(data, { status: response.status })
}

/** 企業ユーザーの有効/無効を切り替える (#1196)。ボディは `{ "disabled": boolean }` */
export async function PATCH(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; userID: string }> },
) {
  const { id, userID } = await params
  const body = await request.text()
  const response = await fetch(
    `${BACKEND_URL}/api/admin/companies/${id}/company-users/${userID}`,
    {
      method: 'PATCH',
      headers: adminHeaders(request),
      body,
    },
  )
  return proxyJson(response)
}
