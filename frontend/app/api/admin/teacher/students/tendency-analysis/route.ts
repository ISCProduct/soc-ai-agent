import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 転送を許可するクエリ。想定外のパラメータをバックエンドへ素通しさせない。
const ALLOWED_PARAMS = ['limit', 'offset', 'q', 'school_id'] as const

// 生徒の傾向分析一覧(Issue #1027)のBFFプロキシ。
//
// school_id はここで検証しない。バックエンドの EchoAdminSchoolScope が
// 担当校リストと照合して範囲外を403で弾くため、改竄経路にはならない。
export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const forwarded = new URLSearchParams()
  for (const key of ALLOWED_PARAMS) {
    const value = searchParams.get(key)
    if (value !== null) forwarded.set(key, value)
  }
  const qs = forwarded.toString()

  try {
    const res = await fetch(
      `${BACKEND_URL}/api/admin/teacher/students/tendency-analysis${qs ? '?' + qs : ''}`,
      {
        cache: 'no-store',
        headers: {
          'X-Admin-Email': request.headers.get('x-admin-email') || '',
          'X-Admin-Token': request.headers.get('x-admin-token') || '',
        },
      },
    )
    // バックエンド停止・502・空ボディでも res.json() で throw させない。
    // throw すると Next が 500 を返し、画面には汎用エラーしか出なくなる。
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
    console.error('teacher student insights proxy error:', error)
    return NextResponse.json({ error: 'バックエンドへの接続に失敗しました' }, { status: 502 })
  }
}
