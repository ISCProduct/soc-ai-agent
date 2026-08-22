import { NextRequest, NextResponse } from 'next/server'
import {
  adminProxyHeaders,
  jsonFromProxyResult,
  proxyAdminBackend,
  proxyErrorResponse,
} from '@/lib/admin-backend-proxy'

/** fetch-missing-batch と同じ。100社でも HeadersTimeout=5分で切れないようにする */
const BATCH_TIMEOUT_MS = 900_000

export const dynamic = 'force-dynamic'
export const maxDuration = 900

export async function POST(request: NextRequest) {
  const body = (await request.text()) || '{}'
  try {
    const result = await proxyAdminBackend('POST', '/api/admin/companies/warm-l1', {
      headers: adminProxyHeaders(request.headers),
      body,
      timeoutMs: BATCH_TIMEOUT_MS,
    })
    return jsonFromProxyResult(result)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    const timedOut = /timeout/i.test(message)
    if (timedOut) {
      return NextResponse.json(
        {
          error:
            'マッチング用データの更新がタイムアウトしました。時間をおいて再度お試しください。途中まで保存されている場合があります。',
        },
        { status: 504 },
      )
    }
    return proxyErrorResponse(err)
  }
}
