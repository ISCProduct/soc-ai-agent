import http from 'node:http'
import https from 'node:https'
import { URL } from 'node:url'
import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

/** Backend がレスポンスを返すまで最大15分待つ（global fetch / undici 既定の HeadersTimeout=5分を回避） */
const BATCH_TIMEOUT_MS = 900_000

export const dynamic = 'force-dynamic'
export const maxDuration = 900

function postWithLongTimeout(
  url: string,
  headers: Record<string, string>,
  body: string,
): Promise<{ status: number; text: string }> {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url)
    const lib = parsed.protocol === 'https:' ? https : http
    const req = lib.request(
      {
        protocol: parsed.protocol,
        hostname: parsed.hostname,
        port: parsed.port || (parsed.protocol === 'https:' ? 443 : 80),
        path: `${parsed.pathname}${parsed.search}`,
        method: 'POST',
        headers: {
          ...headers,
          'Content-Length': Buffer.byteLength(body),
        },
        timeout: BATCH_TIMEOUT_MS,
      },
      (res) => {
        const chunks: Buffer[] = []
        res.on('data', (chunk: Buffer) => chunks.push(chunk))
        res.on('end', () => {
          resolve({
            status: res.statusCode ?? 502,
            text: Buffer.concat(chunks).toString('utf8'),
          })
        })
      },
    )
    req.setTimeout(BATCH_TIMEOUT_MS, () => {
      req.destroy(new Error('Headers/body timeout'))
    })
    req.on('timeout', () => {
      req.destroy(new Error('Request timeout'))
    })
    req.on('error', reject)
    req.write(body)
    req.end()
  })
}

export async function POST(request: NextRequest) {
  const body = (await request.text()) || '{}'
  try {
    const { status, text } = await postWithLongTimeout(
      `${BACKEND_URL}/api/admin/companies/fetch-missing-batch`,
      {
        'Content-Type': 'application/json',
        'X-Admin-Email': request.headers.get('x-admin-email') || '',
        'X-Admin-Token': request.headers.get('x-admin-token') || '',
      },
      body,
    )
    let data: Record<string, unknown> = {}
    if (text) {
      try {
        data = JSON.parse(text) as Record<string, unknown>
      } catch {
        data = status < 400 ? { message: text } : { error: text }
      }
    }
    return NextResponse.json(data, { status })
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err)
    const timedOut = /timeout/i.test(message)
    return NextResponse.json(
      {
        error: timedOut
          ? 'まとめて取得がタイムアウトしました。件数を減らすか、時間をおいて再度お試しください。'
          : `バックエンドに接続できません: ${message}`,
      },
      { status: timedOut ? 504 : 503 },
    )
  }
}
