import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    const documentId = searchParams.get('document_id')
    if (!documentId) {
      return NextResponse.json({ error: 'document_id is required' }, { status: 400 })
    }

    const upstreamHeaders: HeadersInit = {}
    const range = request.headers.get('range')
    if (range) {
      upstreamHeaders.Range = range
    }

    const response = await fetch(`${BACKEND_URL}/api/resume/annotated?document_id=${documentId}`, {
      headers: upstreamHeaders,
    })
    if (!response.ok) {
      return buildProxyJsonResponse(response)
    }

    const buffer = await response.arrayBuffer()
    const headers = new Headers()
    headers.set('Content-Type', response.headers.get('content-type') || 'application/pdf')
    const disposition = response.headers.get('content-disposition')
    if (disposition) {
      headers.set('Content-Disposition', disposition)
    }

    const contentRange = response.headers.get('content-range')
    if (contentRange) {
      headers.set('Content-Range', contentRange)
    }

    const status = response.status === 206 ? 206 : 200
    return new NextResponse(buffer, { status, headers })
  } catch (error) {
    console.error('Resume annotated proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
