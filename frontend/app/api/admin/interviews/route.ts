import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const params = new URLSearchParams()
  for (const key of ['page', 'limit']) {
    const v = searchParams.get(key)
    if (v !== null) params.set(key, v)
  }
  const url = `${BACKEND_URL}/api/admin/interviews?${params}`
  const response = await fetch(url, {
    headers: { 'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '' },
  })
  const raw = await response.text()
  let data: any = {}
  if (raw) {
    try {
      data = JSON.parse(raw)
    } catch {
      data = response.ok ? {} : { error: raw.trim() }
    }
  }
  return NextResponse.json(data, { status: response.status })
}
