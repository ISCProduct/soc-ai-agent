import { NextRequest, NextResponse } from 'next/server'
import { isValidId, proxyStudentConsultation } from '@/lib/consultation-proxy'

export const dynamic = 'force-dynamic'

// 学生本人のスレッドへの返信（#2028）
export async function POST(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  if (!isValidId(id)) return NextResponse.json({ error: 'id が不正です' }, { status: 400 })
  return proxyStudentConsultation(request, `/${id}/messages`, 'POST')
}
