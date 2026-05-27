import { NextRequest } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

function userHeaders(request: NextRequest): Record<string, string> {
  return {
    'X-User-ID': request.headers.get('X-User-ID') || '',
    'X-User-Token': request.headers.get('X-User-Token') || '',
  }
}

export async function POST(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const documentId = searchParams.get('document_id')
  if (!documentId) {
    return new Response('document_id is required', { status: 400 })
  }

  const body = await request.text()

  const response = await fetch(
    `${BACKEND_URL}/api/resume/review/stream?document_id=${documentId}`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...userHeaders(request) },
      body: body || undefined,
    }
  )

  if (!response.ok) {
    const text = await response.text()
    return new Response(text, { status: response.status })
  }

  return new Response(response.body, {
    status: response.status,
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection': 'keep-alive',
      'X-Accel-Buffering': 'no',
    },
  })
}
