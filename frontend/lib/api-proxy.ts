import { timingSafeEqual } from 'node:crypto'
import { NextRequest, NextResponse } from 'next/server'
import { parseProxyResponse, type ParsedProxyResponse, type ProxyResponseData } from '@/lib/proxy-response'
import { looksLikeHtml, userFacingApiMessage } from '@/lib/user-facing-error'

export interface ProxyErrorBody {
  error: string
  status: number
  detail?: string
}

function getString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined
  }
  const trimmed = value.trim()
  return trimmed.length > 0 ? trimmed : undefined
}

function isProxyErrorObject(data: ParsedProxyResponse): data is ProxyResponseData {
  return !!data && typeof data === 'object' && !Array.isArray(data)
}

function getDetailText(data: ParsedProxyResponse, raw: string): string | undefined {
  const detail = isProxyErrorObject(data)
    ? getString(data.detail) ??
      getString(data.details) ??
      getString(data.message) ??
      getString(raw)
    : getString(raw)
  return detail
}

/**
 * Backend の「IP単位」レート制限へ実クライアントIPを引き継ぐヘッダーを作る (#1407)。
 *
 * BFF が転送しないと Backend から見た送信元が frontend タスクの出口IP1つに収束し、
 * ログイン(20回/分)・パスワードリセット(5回/時)・企業情報投稿(5回/時)が
 * 全利用者合計の上限として効いてしまう。
 *
 * 信頼境界:
 * - 実IPの出所は ALB / CloudFront / nginx が追記した `X-Forwarded-For` の**末尾側**だけ。
 *   クライアントが送ってきた `X-Client-IP` や XFF の先頭は一切見ない
 *   （見ると誰でも自分のIPを詐称して、IP単位の制限を無限に分散できる）。
 * - Backend の ALB はインターネット直結なので、Backend 側は BFF と共有する
 *   `BFF_INTERNAL_TOKEN` が一致したときだけ `X-Client-IP` を採用する。
 * - トークン未設定（ローカル開発・シークレット未配布）なら何も送らず、従来どおり
 *   BFF の出口IPで集計される。サイトを落とさないための無効化であって、素通しではない。
 */
export function clientIpHeaders(request: NextRequest): Record<string, string> {
  const token = process.env.BFF_INTERNAL_TOKEN?.trim()
  if (!token) return {}

  const forwardedFor = request.headers.get('x-forwarded-for') ?? ''
  if (!forwardedFor.trim()) return {}
  const hops = trustedProxyHops(request)
  if (hops < 1) return {}
  const parts = forwardedFor.split(',')
  // 末尾から数えて hops 番目が実クライアントIP。
  // 範囲外（想定より手前で止まっている＝経路が想定と違う）なら推測せず転送しない。
  const clientIp = parts[parts.length - hops]?.trim() ?? ''
  if (!clientIp) return {}
  return { 'X-Client-IP': clientIp, 'X-Internal-Token': token }
}

/** CloudFront がオリジンリクエストへ必ず付け直す、経路証明用のヘッダー名。 */
const originTokenHeader = 'x-origin-token'

function tokenMatches(provided: string, expected: string): boolean {
  const a = Buffer.from(provided)
  const b = Buffer.from(expected)
  // timingSafeEqual は長さが違うと例外になるので、先に長さで弾く（長さ自体は秘密ではない）
  return a.length === b.length && timingSafeEqual(a, b)
}

/**
 * frontend の手前にいる「必ず通る」プロキシの段数。信用できないときは 0（=転送しない）。
 *
 * ALB・CloudFront・nginx はいずれも XFF の末尾へ「直前の送信元」を追記するため、
 * 実クライアントIPの位置は段数で決まる。
 * - ALB のみ: 1（既定）
 * - ALB + edge nginx（staging。nginx へは ALB の SG からしか届かない）: 2
 * - CloudFront + ALB（本番）: 2
 *
 * ただし「段数が合っている」だけでは足りない。本番の ALB は 0.0.0.0/0 に開いており、
 * CloudFront を通さず ALB へ直接 `X-Forwarded-For: <詐称IP>` を投げれば、ALB が実IPを
 * 末尾へ足して要素数2の XFF がそのまま出来上がる。段数だけを見ると詐称した先頭を
 * 実IPとして内部トークン付きで Backend へ署名してしまい、IP単位の制限を任意に分散できる。
 * そこで CloudFront がオリジンへ付与する共有シークレット(`CLOUDFRONT_ORIGIN_TOKEN`)で
 * 「本当に CloudFront を通ったか」を確かめてから段数を信用する。
 * CloudFront のカスタムオリジンヘッダーはビューアーが同名ヘッダーを送っても必ず上書き
 * されるため、ビューアー側から詐称できない。
 *
 * `CLOUDFRONT_ORIGIN_TOKEN` 未設定の環境（staging / ローカル）は、手前の段が
 * SG で経路を強制されている前提なので段数をそのまま使う。
 *
 * 多く見積もると詐称値を拾いうるので、不正値は既定の1へ倒す。
 */
function trustedProxyHops(request: NextRequest): number {
  const configured = Number(process.env.TRUSTED_PROXY_HOPS)
  const hops = Number.isInteger(configured) && configured >= 1 ? configured : 1

  const expected = process.env.CLOUDFRONT_ORIGIN_TOKEN?.trim()
  if (!expected) return hops
  const provided = request.headers.get(originTokenHeader)?.trim() ?? ''
  return tokenMatches(provided, expected) ? hops : 0
}

export function extractUserAuthHeaders(request: NextRequest): Record<string, string> {
  // IP単位のレート制限を利用者ごとに効かせるため、認証ヘッダーと同じ経路で引き継ぐ(#1407)
  const headers: Record<string, string> = clientIpHeaders(request)
  const xUserId = request.headers.get('X-User-ID')
  const xUserToken = request.headers.get('X-User-Token')
  const xTenantSlug = request.headers.get('X-Tenant-Slug')
  // middleware.ts が採番したリクエストIDをBackend/RAGまで引き継ぐ(#1188)
  const xRequestId = request.headers.get('X-Request-ID')
  if (xUserId) headers['X-User-ID'] = xUserId
  if (xUserToken) headers['X-User-Token'] = xUserToken
  if (xTenantSlug) headers['X-Tenant-Slug'] = xTenantSlug
  if (xRequestId) headers['X-Request-ID'] = xRequestId
  return headers
}

export async function buildProxyJsonResponse(response: Response): Promise<NextResponse> {
  const raw = await response.text()
  const data = parseProxyResponse(raw, response.ok)

  if (response.ok) {
    return NextResponse.json(data, { status: response.status })
  }

  const rawForUser = looksLikeHtml(raw) ? '' : raw
  const error = userFacingApiMessage(response.status, raw)
  const detail = looksLikeHtml(raw) ? undefined : getDetailText(data, rawForUser)
  const body: ProxyErrorBody = {
    error,
    status: response.status,
    ...(detail && detail !== error && !looksLikeHtml(detail) ? { detail } : {}),
  }
  return NextResponse.json(body, { status: response.status })
}

export function buildProxyNetworkErrorResponse(error: unknown, message: string): NextResponse {
  const detail = error instanceof Error ? error.message : String(error)
  const body: ProxyErrorBody = {
    error: message,
    status: 500,
    ...(detail ? { detail } : {}),
  }
  return NextResponse.json(body, { status: 500 })
}
