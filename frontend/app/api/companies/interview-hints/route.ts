import { NextRequest, NextResponse } from 'next/server'

const RAG_REVIEW_URL = process.env.RAG_REVIEW_URL || 'http://rag-review:9000'
const BACKEND_URL = process.env.BACKEND_URL || process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:8080'

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

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
    const body: unknown = await request.json()
    if (!isRecord(body) || typeof body.company_name !== 'string' || typeof body.position !== 'undefined' && typeof body.position !== 'string') {
      return NextResponse.json({ error: '企業名または職種が不正です' }, { status: 400 })
    }
    if (!body.company_name.trim()) {
      return NextResponse.json({ error: '企業名を入力してください' }, { status: 400 })
    }

    const companyName = body.company_name.trim()
    const company_context = await fetchCompanyBrief(companyName)

    const response = await fetch(`${RAG_REVIEW_URL}/company/hints`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        company_name: companyName,
        position: body.position || '',
        company_context,
      }),
      signal: AbortSignal.timeout(30000),
    })

    if (!response.ok) {
      return NextResponse.json(
        { error: '企業の面接傾向を取得できませんでした' },
        { status: 502 },
      )
    }

    const data: unknown = await response.json()
    if (
      !isRecord(data)
      || !Array.isArray(data.style_tags)
      || !data.style_tags.every((tag) => typeof tag === 'string')
      || !Array.isArray(data.top_questions)
      || !data.top_questions.every((question) => typeof question === 'string')
    ) {
      return NextResponse.json(
        { error: '企業の面接傾向を正しく読み取れませんでした' },
        { status: 502 },
      )
    }
    return NextResponse.json({
      style_tags: data.style_tags,
      top_questions: data.top_questions,
      company_brief: company_context,
    })
  } catch (error: unknown) {
    const err = error as { cause?: { code?: string } }
    if (err?.cause?.code !== 'ECONNREFUSED') {
      console.error('[API] Interview hints error:', error)
    }
    return NextResponse.json(
      { error: '企業の面接傾向を取得できませんでした' },
      { status: 502 },
    )
  }
}
