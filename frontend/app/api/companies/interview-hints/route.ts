import { NextRequest, NextResponse } from 'next/server'

const RAG_REVIEW_URL = process.env.RAG_REVIEW_URL || 'http://rag-review:9000'

export async function POST(request: NextRequest) {
  try {
    const body = await request.json()
    const { company_name, position } = body

    if (!company_name?.trim()) {
      return NextResponse.json({ style_tags: [], top_questions: [] })
    }

    const response = await fetch(`${RAG_REVIEW_URL}/company/hints`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ company_name: company_name.trim(), position: position || '' }),
      signal: AbortSignal.timeout(30000),
    })

    if (!response.ok) {
      return NextResponse.json({ style_tags: [], top_questions: [] }, { status: response.status })
    }

    const data = await response.json()
    return NextResponse.json(data)
  } catch (error) {
    console.error('[API] Interview hints error:', error)
    return NextResponse.json({ style_tags: [], top_questions: [] }, { status: 500 })
  }
}
