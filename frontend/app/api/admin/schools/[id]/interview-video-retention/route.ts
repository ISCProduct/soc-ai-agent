import { NextRequest } from 'next/server'
import {
  proxyAdminBackend,
  jsonFromProxyResult,
  proxyErrorResponse,
  adminProxyHeaders,
} from '@/lib/admin/backend-proxy'

export const dynamic = 'force-dynamic'

// 面接動画の保持期間の更新（SOCAIAGENT-671）。値の検証は Backend の UpdateInterviewVideoRetention が行う
export async function PUT(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const body = await request.text()
  try {
    const result = await proxyAdminBackend(
      'PUT',
      `/api/admin/schools/${encodeURIComponent(id)}/interview-video-retention`,
      {
        headers: adminProxyHeaders(request.headers),
        body,
        timeoutMs: 15_000,
      },
    )
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
