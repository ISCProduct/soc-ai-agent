import { NextRequest, NextResponse } from 'next/server'
import { clientIpHeaders } from '@/lib/api-proxy'

const API_BASE_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()
    const response = await fetch(`${API_BASE_URL}/api/companies/validate`, {
      method: 'POST',
      // ゲストAIのIP単位制限を利用者ごとに効かせる(#1407)
      headers: { 'Content-Type': 'application/json', ...clientIpHeaders(request) },
      body: JSON.stringify(body),
      cache: 'no-store',
    })

    const text = await response.text()
    let data: unknown = {}
    try {
      data = text ? JSON.parse(text) : {}
    } catch {
      data = { error: text || 'validate failed' }
    }

    return NextResponse.json(data, { status: response.status })
  } catch (error) {
    console.error('[API] Company validate error:', error)
    return NextResponse.json({ error: '企業の実在確認に失敗しました' }, { status: 500 })
  }
}
