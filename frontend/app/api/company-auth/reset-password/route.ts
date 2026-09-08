import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

// パスワード再設定を実行する（認証不要）。#1196
//
// Cookie はここでは設定しない。accept-invite と同じく素通しにして、
// クライアント側の persistCompanyAuth が POST /api/company-auth/session を
// 叩いて httpOnly Cookie を張る一本道に揃える。
// ここでも張ると同じ Cookie が2経路から設定され、後で挙動を追いにくくなる。
export async function POST(request: NextRequest) {
  const body = await request.text()
  const res = await fetch(`${BACKEND_URL}/api/company-auth/reset-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body,
  })
  const text = await res.text()
  return new NextResponse(text, {
    status: res.status,
    headers: { 'Content-Type': res.headers.get('Content-Type') || 'application/json' },
  })
}
