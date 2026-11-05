import { NextRequest } from 'next/server'
import {
  proxyAdminBackend,
  jsonFromProxyResult,
  proxyErrorResponse,
  adminProxyHeaders,
} from '@/lib/admin/backend-proxy'

export const dynamic = 'force-dynamic'

// ES添削の履歴（#1531）。担当校スコープは Backend 側で掛かるため、
// school_id はクエリをそのまま渡す。
export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  // バックエンドのパスへ埋めるので、数字以外は通さない。
  if (!/^\d+$/.test(id)) {
    return new Response(JSON.stringify({ error: 'invalid student id' }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    })
  }
  const search = request.nextUrl.search
  try {
    const result = await proxyAdminBackend('GET', `/api/admin/teacher/students/${id}/es-reviews${search}`, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
