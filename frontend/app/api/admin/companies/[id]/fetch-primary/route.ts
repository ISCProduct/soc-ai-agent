import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

/**
 * 主3種（基本・技術・ビジネス関係）を1リクエストで取得する BFF。
 * Backend: POST /api/admin/companies/:id/fetch-primary
 */
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params
  const force = request.nextUrl.searchParams.get('force')
  const qs = force === 'true' ? '?force=true' : ''
  let response: Response
  try {
    response = await fetch(`${BACKEND_URL}/api/admin/companies/${id}/fetch-primary${qs}`, {
      method: 'POST',
      headers: {
        'X-Admin-Email': request.headers.get('x-admin-email') || '',
        'X-Admin-Token': request.headers.get('x-admin-token') || '',
      },
      // 基本 + 技術 + 関係で長時間になりうる
      signal: AbortSignal.timeout(300_000),
    })
  } catch (error) {
    const message =
      error instanceof Error && error.name === 'TimeoutError'
        ? '企業情報の取得がタイムアウトしました'
        : '企業情報取得中に通信エラーが発生しました'
    return NextResponse.json({ error: message }, { status: 504 })
  }
  const raw = await response.text()
  let data: Record<string, unknown> = {}
  if (raw) {
    try {
      data = JSON.parse(raw)
    } catch {
      data = response.ok ? { message: raw } : { error: raw }
    }
  }
  return NextResponse.json(data, { status: response.status })
}
