import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 生徒の傾向分析一覧(Issue #1027)のBFFプロキシ。
// limit / offset / q / school_id をそのままバックエンドへ転送する。
export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const qs = searchParams.toString()

  const res = await fetch(
    `${BACKEND_URL}/api/admin/teacher/students/tendency-analysis${qs ? '?' + qs : ''}`,
    {
      headers: {
        'X-Admin-Email': request.headers.get('x-admin-email') || '',
        'X-Admin-Token': request.headers.get('x-admin-token') || '',
      },
    },
  )
  const data = await res.json()
  return NextResponse.json(data, { status: res.status })
}
