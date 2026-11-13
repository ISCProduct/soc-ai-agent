import { NextRequest, NextResponse } from 'next/server'
import { proxyAdminBackend, jsonFromProxyResult, proxyErrorResponse, adminProxyHeaders } from '@/lib/admin/backend-proxy'

export const dynamic = 'force-dynamic'

// 先生が相談のスレッドを見る（#2028）。担当校以外は Backend が 403 を返す
export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  if (!/^\d+$/.test(id)) return NextResponse.json({ error: 'id が不正です' }, { status: 400 })
  try {
    const result = await proxyAdminBackend('GET', `/api/admin/teacher/consultations/${id}`, {
      headers: adminProxyHeaders(request.headers),
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
