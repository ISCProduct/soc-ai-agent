import { NextRequest, NextResponse } from 'next/server'
import { buildProxyJsonResponse, buildProxyNetworkErrorResponse, extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

/**
 * レビュー画面が書類そのものを描くための、PDFに揃えた元の書類。
 * PDF.js が Range で必要な部分だけ読めるよう、Range を上流へ渡す。
 */
export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url)
    const documentId = searchParams.get('document_id')
    if (!documentId || !/^\d+$/.test(documentId)) {
      return NextResponse.json({ error: 'document_id is required' }, { status: 400 })
    }

    const upstreamHeaders = extractUserAuthHeaders(request)
    const range = request.headers.get('range')
    if (range) {
      upstreamHeaders.Range = range
    }

    const response = await fetch(`${BACKEND_URL}/api/resume/document?document_id=${encodeURIComponent(documentId)}`, {
      headers: upstreamHeaders,
    })
    if (!response.ok) {
      return buildProxyJsonResponse(response)
    }

    const buffer = await response.arrayBuffer()
    const headers = new Headers()
    headers.set('Content-Type', response.headers.get('content-type') || 'application/pdf')
    // 本人の書類なので、共有キャッシュに残さない
    headers.set('Cache-Control', 'private, no-store')
    const contentRange = response.headers.get('content-range')
    if (contentRange) {
      headers.set('Content-Range', contentRange)
    }
    // これが無いと PDF.js は分割読みを始めず、書類全体を受け取る
    const acceptRanges = response.headers.get('accept-ranges')
    if (acceptRanges) {
      headers.set('Accept-Ranges', acceptRanges)
    }
    headers.set('Content-Length', String(buffer.byteLength))

    const status = response.status === 206 ? 206 : 200
    return new NextResponse(buffer, { status, headers })
  } catch (error) {
    console.error('Resume document proxy error:', error)
    return buildProxyNetworkErrorResponse(error, 'Failed to connect to backend')
  }
}
