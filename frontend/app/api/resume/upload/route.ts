import { NextRequest, NextResponse } from 'next/server'
import { parseProxyResponse } from '@/lib/proxy-response'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function POST(request: NextRequest) {
  try {
    const formData = await request.formData()
    const response = await fetch(`${BACKEND_URL}/api/resume/upload`, {
      method: 'POST',
      body: formData,
    })

    const raw = await response.text()
    const data = parseProxyResponse(raw, response.ok)
    return NextResponse.json(data, { status: response.status })
  } catch (error) {
    console.error('Resume upload proxy error:', error)
    return NextResponse.json({ error: 'Failed to connect to backend' }, { status: 500 })
  }
}
