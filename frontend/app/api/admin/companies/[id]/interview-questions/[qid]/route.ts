import { NextRequest, NextResponse } from 'next/server'

const BACKEND_URL = process.env.BACKEND_URL || 'http://app:8080'

export const dynamic = 'force-dynamic'

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; qid: string }> },
) {
  const { id, qid } = await params
  const body = await request.json()
  const response = await fetch(`${BACKEND_URL}/api/admin/companies/${id}/interview-questions/${qid}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '',
    },
    body: JSON.stringify(body),
  })
  const data = await response.json().catch(() => ({}))
  return NextResponse.json(data, { status: response.status })
}

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; qid: string }> },
) {
  const { id, qid } = await params
  const response = await fetch(`${BACKEND_URL}/api/admin/companies/${id}/interview-questions/${qid}`, {
    method: 'DELETE',
    headers: {
      'X-Admin-Email': request.headers.get('x-admin-email') || '',
      'X-Admin-Token': request.headers.get('x-admin-token') || '',
    },
  })
  if (response.status === 204) {
    return new NextResponse(null, { status: 204 })
  }
  const data = await response.json().catch(() => ({}))
  return NextResponse.json(data, { status: response.status })
}
