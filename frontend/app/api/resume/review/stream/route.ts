import { NextRequest } from 'next/server'
import { extractUserAuthHeaders } from '@/lib/api-proxy'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

const UPSTREAM_ERROR_MESSAGE = 'レビューに失敗しました。時間をおいてもう一度お試しください。'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  const { searchParams } = new URL(request.url)
  const documentId = searchParams.get('document_id')
  if (!documentId) {
    return new Response('document_id is required', { status: 400 })
  }

  const body = await request.text()

  // 利用者が画面を離れたら上流（Backend → RAG）の接続も閉じる。上流の失敗は固定文言にして、本文はログだけに残す
  let response: Response
  try {
    response = await fetch(`${BACKEND_URL}/api/resume/review/stream?document_id=${documentId}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...extractUserAuthHeaders(request) },
      body: body || undefined,
      signal: request.signal,
    })
  } catch (err) {
    if (request.signal.aborted) return new Response(null, { status: 499 })
    console.error('resume review stream: upstream fetch failed', err instanceof Error ? err.name : 'unknown')
    return new Response(UPSTREAM_ERROR_MESSAGE, { status: 502 })
  }

  if (!response.ok) {
    const text = await response.text().catch(() => '')
    // 4xx は入力・認可の案内として返す。5xx は内部の文言を出さない
    if (response.status >= 500) {
      console.error('resume review stream: upstream error', response.status, text.slice(0, 200))
      return new Response(UPSTREAM_ERROR_MESSAGE, { status: 502 })
    }
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
