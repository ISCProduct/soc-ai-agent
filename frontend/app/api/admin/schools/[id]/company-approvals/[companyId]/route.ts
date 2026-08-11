import { NextRequest } from 'next/server'
import {
  proxyAdminBackend,
  jsonFromProxyResult,
  proxyErrorResponse,
  adminProxyHeaders,
} from '@/lib/admin-backend-proxy'

export const dynamic = 'force-dynamic'

export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; companyId: string }> },
) {
  const { id, companyId } = await params
  try {
    const result = await proxyAdminBackend(
      'DELETE',
      `/api/admin/schools/${id}/company-approvals/${companyId}`,
      { headers: adminProxyHeaders(request.headers), timeoutMs: 15_000 },
    )
    return jsonFromProxyResult(result)
  } catch (err) {
    return proxyErrorResponse(err)
  }
}
