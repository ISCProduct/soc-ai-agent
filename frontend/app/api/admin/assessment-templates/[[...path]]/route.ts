import { NextRequest, NextResponse } from 'next/server'
import { adminProxyHeaders } from '@/lib/admin/backend-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

type RouteContext = { params: Promise<{ path?: string[] }> }

/** 学校独自の適性検査（#1675）の BFF プロキシ。許可する経路は「一覧・作成」と「:id/students」だけ。 */
function targetPath(path: string[]): string | null {
  if (path.length === 0) return ''
  if (path.length === 2 && /^\d+$/.test(path[0]) && path[1] === 'students') return `/${path[0]}/students`
  return null
}

// school_id の検証はここでしない。Backend の EchoAdminSchoolScope が担当校と照合して 403 にする。
async function proxy(request: NextRequest, context: RouteContext) {
  const suffix = targetPath((await context.params).path ?? [])
  if (suffix === null) return NextResponse.json({ error: 'Not Found' }, { status: 404 })

  const { searchParams } = new URL(request.url)
  const schoolId = searchParams.get('school_id')
  const qs = schoolId !== null ? `?school_id=${encodeURIComponent(schoolId)}` : ''
  const hasBody = request.method === 'POST'
  try {
    const res = await fetch(`${BACKEND_URL}/api/admin/assessment-templates${suffix}${qs}`, {
      method: request.method,
      cache: 'no-store',
      headers: adminProxyHeaders(request.headers),
      body: hasBody ? await request.text() : undefined,
    })
    // バックエンド停止・空ボディでも res.json() で throw させない
    const raw = await res.text()
    let data: unknown = {}
    if (raw) {
      try {
        data = JSON.parse(raw)
      } catch {
        data = res.ok ? { message: raw } : { error: raw }
      }
    }
    return NextResponse.json(data, { status: res.status })
  } catch (error) {
    console.error('assessment-templates proxy error:', error)
    return NextResponse.json({ error: 'バックエンドへの接続に失敗しました' }, { status: 502 })
  }
}

export const GET = proxy
export const POST = proxy
