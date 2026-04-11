import { NextRequest, NextResponse } from 'next/server'

export const dynamic = 'force-dynamic'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function POST(request: NextRequest) {
  const body = await request.json().catch(() => ({}))

  let response: Response
  try {
    response = await fetch(`${BACKEND_URL}/api/admin/company-graph/crawl`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Admin-Email': request.headers.get('X-Admin-Email') ?? '',
        'X-Admin-Token': request.headers.get('X-Admin-Token') ?? '',
      },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(310_000), // 5 min + buffer
    })
  } catch (err: any) {
    return NextResponse.json(
      { ok: false, error: `バックエンドサービスに接続できません: ${err?.message ?? err}` },
      { status: 503 },
    )
  }

  const data = await response.json().catch(() => ({ ok: false, error: 'Invalid response' }))
  return NextResponse.json(data, { status: response.ok ? 200 : response.status })
}

/** 年度の自動計算（Go バックエンドから取得）*/
export async function GET() {
  try {
    const res = await fetch(`${BACKEND_URL}/api/admin/company-graph/target-year`, {
      signal: AbortSignal.timeout(3000),
    })
    if (res.ok) {
      return NextResponse.json(await res.json())
    }
  } catch {
    // fall through to local calculation
  }

  const today = new Date()
  const month = today.getMonth() + 1
  const year = today.getFullYear()
  const targetYear = month >= 4 ? year + 2 : year + 1
  return NextResponse.json({ target_year: targetYear })
}
