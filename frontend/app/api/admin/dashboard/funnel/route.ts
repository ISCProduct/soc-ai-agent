import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

// 機能横断のコンバージョンファネル（#2039）。担当校スコープは Backend が検証する。
export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const qs = searchParams.toString()

  const res = await fetch(`${BACKEND_URL}/api/admin/dashboard/funnel${qs ? '?' + qs : ''}`, {
    headers: {
      'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '',
    },
  })
  // 解析できない応答を 200 の空データとして返すと、画面が結果もエラーも出さなくなる
  const data: unknown = await res.json().catch(() => undefined)
  if (data === undefined) {
    return NextResponse.json({ message: 'invalid response from backend' }, { status: 502 })
  }
  return NextResponse.json(data, { status: res.status })
}
