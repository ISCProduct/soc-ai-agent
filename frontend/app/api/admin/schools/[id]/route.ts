import { NextRequest } from 'next/server'
import {
  proxyAdminBackend,
  jsonFromProxyResult,
  proxyErrorResponse,
  adminProxyHeaders,
} from '@/lib/admin/backend-proxy'

export const dynamic = 'force-dynamic'

// 学校1件の取得（面接動画の保持期間カードが現在値を読む, SOCAIAGENT-671）
export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  try {
    const result = await proxyAdminBackend('GET', `/api/admin/schools/${encodeURIComponent(id)}`, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
