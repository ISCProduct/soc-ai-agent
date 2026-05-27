import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

function userHeaders(request: NextRequest): Record<string, string> {
  return {
    'X-User-ID': request.headers.get('X-User-ID') || '',
    'X-User-Token': request.headers.get('X-User-Token') || '',
  }
}

export async function POST(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    const documentId = searchParams.get('document_id')
    if (!documentId) {
      return NextResponse.json({ error: 'document_id is required' }, { status: 400 })
    }

    const body = await request.text()
    const headers: Record<string, string> = userHeaders(request)
    if (body) {
      headers['Content-Type'] = 'application/json'
    }

    const response = await fetch(`${BACKEND_URL}/api/resume/review?document_id=${documentId}`, {
      method: 'POST',
      headers,
      body: body || undefined,
    })
    return buildProxyJsonResponse(response)
  } catch (error) {
    console.error('Resume review proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
