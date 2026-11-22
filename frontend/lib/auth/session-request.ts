import { NextRequest, NextResponse } from 'next/server'

type SessionPostResult =
  | { ok: true; body: Record<string, unknown> }
  | { ok: false; response: NextResponse }

function reject(route: string, reason: string, status: number): SessionPostResult {
  // 値（トークン・Origin）はログに出さない。理由の種類だけ残す。
  console.warn(`[session] ${route} rejected reason=${reason}`)
  return { ok: false, response: NextResponse.json({ error: 'invalid request' }, { status }) }
}

// セッション Cookie を発行する POST を、自サイトの fetch からの呼び出しに限る (SOCAIAGENT-684)。
// application/json 必須で text/plain のフォーム送信を落とし、Origin で他サイトからの送信を落とす。
export async function parseSessionPost(
  request: NextRequest,
  route: string
): Promise<SessionPostResult> {
  const contentType = request.headers.get('content-type') ?? ''
  if (!contentType.toLowerCase().startsWith('application/json')) {
    return reject(route, 'content-type', 415)
  }

  const origin = request.headers.get('origin')
  const host = request.headers.get('host') ?? request.nextUrl.host
  let originHost = ''
  try {
    originHost = origin ? new URL(origin).host : ''
  } catch {
    // 不正な Origin は不一致として扱う
  }
  if (!originHost || originHost !== host) {
    return reject(route, 'origin', 403)
  }

  try {
    const body: unknown = await request.json()
    if (typeof body !== 'object' || body === null || Array.isArray(body)) {
      return reject(route, 'json', 400)
    }
    return { ok: true, body: body as Record<string, unknown> }
  } catch {
    return reject(route, 'json', 400)
  }
}
