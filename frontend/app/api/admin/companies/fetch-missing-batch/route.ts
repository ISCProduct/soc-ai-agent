import { NextRequest, NextResponse } from 'next/server'
import {
  adminProxyHeaders,
  jsonFromProxyResult,
  proxyAdminBackend,
  proxyErrorResponse,
} from '@/lib/admin-backend-proxy'

/** Backend がレスポンスを返すまで最大15分待つ（global fetch / undici 既定の HeadersTimeout=5分を回避） */
const BATCH_TIMEOUT_MS = 900_000

export const dynamic = 'force-dynamic'
export const maxDuration = 900

export async function POST(request: NextRequest) {
  const body = (await request.text()) || '{}'
  try {
    const result = await proxyAdminBackend('POST', '/api/admin/companies/fetch-missing-batch', {
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
            'まとめて取得がタイムアウトしました。件数を減らすか、時間をおいて再度お試しください。',
        },
        { status: 504 },
      )
    }
    return proxyErrorResponse(err)
  }
}
