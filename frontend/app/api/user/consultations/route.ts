import { NextRequest } from 'next/server'
import { proxyStudentConsultation } from '@/lib/consultation-proxy'

export const dynamic = 'force-dynamic'

// 学生の相談の一覧（#2028）。limit と cursor だけを Backend へ渡す
export async function GET(request: NextRequest) {
  const qs = new URL(request.url).searchParams
  const params = new URLSearchParams()
  for (const key of ['limit', 'cursor']) {
    const v = qs.get(key)
    if (v) params.set(key, v)
  }
  return proxyStudentConsultation(request, params.size ? `?${params}` : '', 'GET')
}

// 学生から先生への質問（#2028）
export async function POST(request: NextRequest) {
  return proxyStudentConsultation(request, '', 'POST')
}
