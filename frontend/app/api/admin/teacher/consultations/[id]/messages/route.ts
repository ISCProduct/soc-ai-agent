import { NextRequest, NextResponse } from 'next/server'
import { proxyAdminBackend, jsonFromProxyResult, proxyErrorResponse, adminProxyHeaders } from '@/lib/admin/backend-proxy'

export const dynamic = 'force-dynamic'

// 先生が相談に返信する（#2028）。担当校以外は Backend が 404 を返す（相談の有無を推測させない）
export async function POST(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  if (!/^\d+$/.test(id)) return NextResponse.json({ error: 'id が不正です' }, { status: 400 })
  const body = await request.text()
  try {
    const result = await proxyAdminBackend('POST', `/api/admin/teacher/consultations/${encodeURIComponent(id)}/messages`, {
      headers: adminProxyHeaders(request.headers),
      body,
      timeoutMs: 15_000,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
