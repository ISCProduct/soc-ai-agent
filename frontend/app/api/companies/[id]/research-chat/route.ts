import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'
import { validCompanyId } from '@/lib/company/correlation-proxy'

export const dynamic = 'force-dynamic'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'
const PASS_THROUGH = new Set([400, 404, 429, 503])

/** POST /api/companies/:id/research-chat → Backend 企業研究AI */
export async function POST(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  if (!validCompanyId(id)) {
    return NextResponse.json({ error: '企業を選び直してください。' }, { status: 400 })
  }
  try {
    const authHeaders = extractUserAuthHeaders(request)
    if (!authHeaders['X-User-Token']) {
      return NextResponse.json({ error: 'ログインが必要です' }, { status: 401 })
    }
    const response = await fetch(`${BACKEND_URL}/api/companies/${id}/research-chat`, {
      method: 'POST',
      headers: { ...authHeaders, 'Content-Type': 'application/json' },
      body: await request.text(),
      cache: 'no-store',
      signal: request.signal,
    })
    if (PASS_THROUGH.has(response.status)) {
      const data = (await response.json().catch(() => ({}))) as { error?: unknown; message?: unknown }
      const error =
        (typeof data.error === 'string' && data.error.trim()) ||
        (typeof data.message === 'string' && data.message.trim()) ||
        '処理に失敗しました。しばらくしてから再試行してください。'
      return NextResponse.json({ error }, { status: response.status })
    }
    return buildProxyJsonResponse(response)
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, '企業研究AIに接続できませんでした。もう一度お試しください。')
  }
}
