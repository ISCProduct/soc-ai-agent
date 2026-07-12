import { NextRequest, NextResponse } from 'next/server'

const RAG_REVIEW_URL = process.env.RAG_REVIEW_URL || 'http://rag-review:9000'
const BACKEND_URL = process.env.BACKEND_URL || process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:8080'

async function fetchCompanyBrief(companyName: string): Promise<string> {
  try {
    const url = `${BACKEND_URL.replace(/\/$/, '')}/api/companies/brief?name=${encodeURIComponent(companyName)}`
    const response = await fetch(url, {
      cache: 'no-store',
      signal: AbortSignal.timeout(8000),
    })
    if (!response.ok) return ''
    const data = (await response.json()) as { brief?: string }
    return typeof data.brief === 'string' ? data.brief.trim() : ''
  } catch {
    return ''
  }
}

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()
    const { company_name, position } = body

    if (!company_name?.trim()) {
      return NextResponse.json({ style_tags: [], top_questions: [] })
    }

    const companyName = company_name.trim()
    const company_context = await fetchCompanyBrief(companyName)

    const response = await fetch(`${RAG_REVIEW_URL}/company/hints`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        company_name: companyName,
        position: position || '',
        company_context,
      }),
      signal: AbortSignal.timeout(30000),
    })

    if (!response.ok) {
      // RAG サービス側のエラーは UI に伝えず空レスポンスで正常扱い
      return NextResponse.json({ style_tags: [], top_questions: [], company_brief: company_context })
    }

    const data = await response.json()
    return NextResponse.json({ ...data, company_brief: company_context })
  } catch (error: unknown) {
    const err = error as { cause?: { code?: string } }
    // RAG 未起動（ECONNREFUSED 等）はログのみで空レスポンスを返す
    if (err?.cause?.code !== 'ECONNREFUSED') {
      console.error('[API] Interview hints error:', error)
    }
    return NextResponse.json({ style_tags: [], top_questions: [] })
  }
}
