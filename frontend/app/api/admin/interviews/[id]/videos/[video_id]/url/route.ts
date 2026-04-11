import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; video_id: string }> }
) {
  const resolvedParams = await params
  const url = `${BACKEND_URL}/api/admin/interviews/${resolvedParams.id}/videos/${resolvedParams.video_id}/url`
  const response = await fetch(url, {
    headers: { 'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '' },
  })
  const data = await response.json().catch(() => ({}))
  return NextResponse.json(data, { status: response.status })
}
