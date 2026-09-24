import { NextRequest, NextResponse } from 'next/server'
import { clientIpHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

// パスワード再設定メールの送信要求をBackendへ転送する（認証不要）。
// アカウントの存在有無を漏らさないため、Backendは常に200を返す仕様 (#1196)
export async function POST(request: NextRequest) {
  const body = await request.text()
  const res = await fetch(`${BACKEND_URL}/api/company-auth/forgot-password`, {
    method: 'POST',
    // パスワードリセットのIP単位制限(5回/時)を利用者ごとに効かせる(#1407)
    headers: { 'Content-Type': 'application/json', ...clientIpHeaders(request) },
    body,
  })
  const text = await res.text()
  return new NextResponse(text, {
    status: res.status,
    headers: { 'Content-Type': res.headers.get('Content-Type') || 'application/json' },
  })
}
