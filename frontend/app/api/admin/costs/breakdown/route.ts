import { NextRequest } from 'next/server'
import {
  adminProxyHeaders,
  jsonFromProxyResult,
  proxyAdminBackend,
  proxyErrorResponse,
} from '@/lib/admin-backend-proxy'

export const dynamic = 'force-dynamic'
export const maxDuration = 30

// 機能別・プロバイダ別・モデル別・組織別のAI利用量内訳（#1294）
export async function GET(request: NextRequest) {
  try {
    const by = request.nextUrl.searchParams.get('by') || 'feature'
    const days = request.nextUrl.searchParams.get('days') || '30'
    const result = await proxyAdminBackend(
      'GET',
      `/api/admin/costs/breakdown?by=${encodeURIComponent(by)}&days=${encodeURIComponent(days)}`,
      {
        headers: adminProxyHeaders(request.headers),
        timeoutMs: 15_000,
      },
    )
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
