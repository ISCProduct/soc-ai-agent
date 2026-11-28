import type { NextRequest } from 'next/server'
import { proxyCorrelation } from '@/lib/company/correlation-proxy'

export const dynamic = 'force-dynamic'
export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  return proxyCorrelation(request, (await params).id, true)
}
