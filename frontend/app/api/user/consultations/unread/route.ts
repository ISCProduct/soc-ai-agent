import { NextRequest } from 'next/server'
import { proxyStudentConsultation } from '@/lib/consultation-proxy'

export const dynamic = 'force-dynamic'

// 学生のスレッドごとの、先生の未読の発言の数（#2028）
export async function GET(request: NextRequest) {
  return proxyStudentConsultation(request, '/unread', 'GET')
}
