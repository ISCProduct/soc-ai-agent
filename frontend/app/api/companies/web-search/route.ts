import { NextRequest, NextResponse } from 'next/server'

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
      headers: { 'Content-Type': 'application/json' },
      cache: 'no-store',
    })

    if (!response.ok) {
      return NextResponse.json(
        { results: [], message: '企業検索に失敗しました。しばらくしてから再度お試しください' },
        { status: response.status },
      )
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('[API] Web search error:', error)
    return NextResponse.json(
      { results: [], message: 'バックエンドに接続できません。BACKEND_URL を確認してください' },
      { status: 502 },
    )
  }
}
