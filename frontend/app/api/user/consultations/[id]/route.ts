import { NextRequest, NextResponse } from 'next/server'
import { isValidId, proxyStudentConsultation } from '@/lib/consultation-proxy'

export const dynamic = 'force-dynamic'

// 学生本人のスレッド（#2028）。他人のスレッドは Backend が 404 を返す
export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  if (!isValidId(id)) return NextResponse.json({ error: 'id が不正です' }, { status: 400 })
  return proxyStudentConsultation(request, `/${id}`, 'GET')
}
