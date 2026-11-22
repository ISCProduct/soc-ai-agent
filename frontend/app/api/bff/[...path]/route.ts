import { NextRequest, NextResponse } from 'next/server'
import { SERVER_BACKEND_URL } from '@/lib/auth/session-cookies'
import { buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

/**
 * 画面が Backend を直接呼んでいた学生向けAPIの中継 (SOCAIAGENT-683)。
 * X-User-Token は middleware が httpOnly Cookie から付ける（extractUserAuthHeaders で引き継ぐ）ので、
 * 画面は JWT を持たなくてよい。中継するのは下の許可リストに合うパスだけ。
 */
const ALLOWED_PATHS = [
  // 本登録。ゲストを引き継ぐとき、画面はもうトークンを持たないので Cookie のセッションを渡す必要がある
  /^auth\/register$/,
  /^interviews(\/|$)/,
  /^user\/(preferences|industries|guidances|training-consent|scouts|scout-blocks)(\/|$)/,
  /^auth\/user$/,
  /^realtime\/token$/,
]

// 応答に載っていても画面へは渡さない項目（トークンは Cookie にだけ置く）
const TOKEN_FIELDS = ['token', 'user_token', 'refresh_token']

function wantsGuestPromotion(body: ArrayBuffer | undefined): boolean {
  if (!body) return false
  try {
    const parsed: unknown = JSON.parse(new TextDecoder().decode(body))
    return typeof parsed === 'object' && parsed !== null && (parsed as Record<string, unknown>).promote_guest === true
  } catch {
    return false
  }
}

async function proxy(request: NextRequest, path: string[]): Promise<NextResponse> {
  const joined = path.join('/')
  if (path.some((seg) => seg === '' || seg === '.' || seg === '..') || !ALLOWED_PATHS.some((re) => re.test(joined))) {
    return NextResponse.json({ error: 'not found' }, { status: 404 })
  }

  const headers: Record<string, string> = extractUserAuthHeaders(request)
  const contentType = request.headers.get('content-type')
  if (contentType) headers['Content-Type'] = contentType

  const hasBody = request.method !== 'GET' && request.method !== 'HEAD'
  // multipart（面接の音声）もそのまま渡す
  const body = hasBody ? await request.arrayBuffer() : undefined

  // 本登録に今のセッションを渡すのは、ゲストを引き継ぐときだけ
  // （通常の登録で渡すと、別アカウントを作りたい場合に引き継ぎ判定へ入ってしまう）
  if (joined === 'auth/register' && !wantsGuestPromotion(body)) {
    delete headers['X-User-ID']
    delete headers['X-User-Token']
  }

  let res: Response
  try {
    res = await fetch(`${SERVER_BACKEND_URL}/api/${joined}${request.nextUrl.search}`, {
      method: request.method,
      headers,
      body,
      cache: 'no-store',
    })
  } catch (error) {
    return buildProxyNetworkErrorResponse(error, 'サーバーに接続できませんでした')
  }

  const resType = res.headers.get('Content-Type') || 'application/json'
  if (joined === 'auth/user' && res.ok && resType.includes('application/json')) {
    const data: unknown = await res.json().catch(() => null)
    if (data && typeof data === 'object') {
      for (const field of TOKEN_FIELDS) delete (data as Record<string, unknown>)[field]
    }
    return NextResponse.json(data, { status: res.status })
  }
  // 面接の multipart 応答（音声）を読み切らずに流す
  return new NextResponse(res.body, { status: res.status, headers: { 'Content-Type': resType } })
}

type RouteContext = { params: Promise<{ path: string[] }> }

export async function GET(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}
export async function POST(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}
export async function PUT(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}
export async function PATCH(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}
export async function DELETE(request: NextRequest, context: RouteContext) {
  return proxy(request, (await context.params).path)
}
