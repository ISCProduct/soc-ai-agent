import { NextRequest, NextResponse } from 'next/server'
import { clientIpHeaders } from '@/lib/api-proxy'

const API_BASE_URL = process.env.BACKEND_URL || 'http://app:8080'

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    const q = searchParams.get('q') || ''

    if (!q.trim()) {
      return NextResponse.json({ results: [] })
    }

    const url = `${API_BASE_URL}/api/companies/web-search?q=${encodeURIComponent(q)}`
    const response = await fetch(url, {
      method: 'GET',
      // ゲストAIのIP単位制限を利用者ごとに効かせる(#1407)
      headers: { 'Content-Type': 'application/json', ...clientIpHeaders(request) },
      cache: 'no-store',
      // ブラウザが諦めたあとも課金済みの検索を続けない
      signal: request.signal,
    })

    if (!response.ok) {
      const text = await response.text()
      let data: unknown
      try {
        data = JSON.parse(text)
      } catch {
        data = { results: [], message: '企業検索に失敗しました。しばらくしてから再度お試しください' }
      }
      return NextResponse.json(data, { status: response.status })
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    if (error instanceof Error && error.name === 'AbortError') {
      return new NextResponse(null, { status: 499 })
    }
    console.error('[API] Web search error:', error)
    return NextResponse.json(
      { results: [], message: 'バックエンドに接続できません。BACKEND_URL を確認してください' },
      { status: 502 },
    )
  }
}
