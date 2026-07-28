import http from 'node:http'
import https from 'node:https'
import { URL } from 'node:url'
import { NextResponse } from 'next/server'

export const ADMIN_BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

type ProxyResult = { status: number; text: string }

function isTransientSocketError(err: unknown): boolean {
  if (!(err instanceof Error)) return false
  const msg = err.message.toLowerCase()
  const code =
    typeof err === 'object' && err !== null && 'code' in err
      ? String((err as { code?: string }).code || '')
      : ''
  return (
    code === 'UND_ERR_SOCKET' ||
    code === 'ECONNRESET' ||
    code === 'ECONNREFUSED' ||
    msg.includes('other side closed') ||
    msg.includes('socket hang up') ||
    msg.includes('econnreset') ||
    msg.includes('econnrefused')
  )
}

function requestOnce(
  method: string,
  url: string,
  headers: Record<string, string>,
  body: string | undefined,
  timeoutMs: number,
): Promise<ProxyResult> {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url)
    const lib = parsed.protocol === 'https:' ? https : http
    const req = lib.request(
      {
        protocol: parsed.protocol,
        hostname: parsed.hostname,
        port: parsed.port || (parsed.protocol === 'https:' ? 443 : 80),
        path: `${parsed.pathname}${parsed.search}`,
        method,
        headers: {
          ...headers,
          ...(body != null ? { 'Content-Length': Buffer.byteLength(body) } : {}),
        },
        timeout: timeoutMs,
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
    req.setTimeout(timeoutMs, () => {
      req.destroy(new Error('Request timeout'))
    })
    req.on('timeout', () => {
      req.destroy(new Error('Request timeout'))
    })
    req.on('error', reject)
    if (body != null) req.write(body)
    req.end()
  })
}

/** Backend への転送。再起動中の切断は1回だけリトライする。 */
export async function proxyAdminBackend(
  method: string,
  path: string,
  options: {
    headers?: Record<string, string>
    body?: string
    timeoutMs?: number
  } = {},
): Promise<ProxyResult> {
  const timeoutMs = options.timeoutMs ?? 180_000
  const url = `${ADMIN_BACKEND_URL}${path}`
  const headers = options.headers ?? {}
  try {
    return await requestOnce(method, url, headers, options.body, timeoutMs)
  } catch (err) {
    if (!isTransientSocketError(err)) throw err
    // air 再起動などで接続が切れた場合、少し待って再試行
    await new Promise((r) => setTimeout(r, 800))
    return requestOnce(method, url, headers, options.body, timeoutMs)
  }
}

export function jsonFromProxyResult(result: ProxyResult): NextResponse {
  let data: Record<string, unknown> = {}
  if (result.text) {
    try {
      data = JSON.parse(result.text) as Record<string, unknown>
    } catch {
      data = result.status < 400 ? { message: result.text } : { error: result.text }
    }
  } else if (result.status >= 400) {
    data = { error: 'バックエンドから空の応答が返りました' }
  }
  return NextResponse.json(data, { status: result.status })
}

export function proxyErrorResponse(err: unknown): NextResponse {
  const message = err instanceof Error ? err.message : String(err)
  const timedOut = /timeout/i.test(message)
  const transient = isTransientSocketError(err)
  return NextResponse.json(
    {
      error: timedOut
        ? '処理がタイムアウトしました。時間をおいて再度お試しください。'
        : transient
          ? 'サーバーが再起動中か一時的に接続できませんでした。数秒待って再度お試しください。'
          : `バックエンドに接続できません: ${message}`,
    },
    { status: timedOut ? 504 : 503 },
  )
}

export function adminProxyHeaders(requestHeaders: Headers): Record<string, string> {
  return {
    'Content-Type': 'application/json',
    'X-Admin-Email': requestHeaders.get('x-admin-email') || '',
    'X-Admin-Token': requestHeaders.get('x-admin-token') || '',
  }
}
