'use client'

// 学校ごとの企業掲載承認（取得とトグル）を受け持つ。
import { useEffect, useState } from 'react'
import { authService } from '@/lib/auth'
import { adminFetchJson, toAdminErrorMessage } from '@/lib/admin/fetch'
import { canToggleApproval } from '@/lib/admin/company-approval'

export function useCompanyApprovals(schoolId: number | undefined) {
  // 学校の承認済み企業ID。null は「取得できていない」。空集合と区別する(#1452)。
  // 以前は失敗時も空集合のままで、承認済みの企業まで一覧が全件「未承認」に見えていた。
  const [approvedCompanyIds, setApprovedCompanyIds] = useState<Set<number> | null>(null)
  const [approvalBusyId, setApprovalBusyId] = useState<number | null>(null)
  const [approvalError, setApprovalError] = useState('')
  // 取得中。取得失敗（null）と同じ「不明」表示にしないために分ける（#2573）
  const [approvalLoading, setApprovalLoading] = useState(false)

  useEffect(() => {
    if (schoolId === undefined) {
      setApprovedCompanyIds(null)
      setApprovalError('')
      setApprovalLoading(false)
      return
    }
    let cancelled = false
    const load = async () => {
      setApprovalError('')
      setApprovalLoading(true)
      try {
        // adminFetchJson は非2xx・タイムアウト・ALB の HTML エラーページを
        // すべて日本語メッセージ付きの例外へ正規化する(#1066)。素の fetch だと
        // 500 が HTML を返したとき .json() が例外になり、空集合のまま
        // 「全件未承認」に見えてしまう。
        const data = await adminFetchJson<{ company_ids?: number[] }>(
          `/api/admin/schools/${schoolId}/company-approvals`,
          { headers: authService.getAdminFetchHeaders(), cache: 'no-store' },
          '承認状態の取得に失敗しました',
        )
        if (!cancelled) setApprovedCompanyIds(new Set<number>(data?.company_ids || []))
      } catch (e) {
        if (cancelled) return
        setApprovedCompanyIds(null)
        setApprovalError(toAdminErrorMessage(e))
      } finally {
        if (!cancelled) setApprovalLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [schoolId])

  const toggleCompanyApproval = async (companyId: number, approved: boolean) => {
    // 承認状態が取れていないときは操作させない。サーバー側の実態と食い違うため。
    if (!canToggleApproval(approvedCompanyIds, schoolId)) return
    setApprovalBusyId(companyId)
    setApprovalError('')
    try {
      await adminFetchJson(
        `/api/admin/schools/${schoolId}/company-approvals${approved ? `/${companyId}` : ''}`,
        {
          method: approved ? 'DELETE' : 'POST',
          headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
          body: approved ? undefined : JSON.stringify({ company_id: companyId }),
        },
        approved ? '承認の解除に失敗しました' : '承認に失敗しました',
      )
      setApprovedCompanyIds((prev) => {
        const next = new Set(prev ?? [])
        if (approved) next.delete(companyId)
        else next.add(companyId)
        return next
      })
    } catch (e) {
      // 無言で返すと、押しても反応しないチップを何度も押すことになる(#1452)。
      // 成功していないので approvedCompanyIds は変えない。
      setApprovalError(toAdminErrorMessage(e))
    } finally {
      setApprovalBusyId(null)
    }
  }

  return { approvedCompanyIds, approvalLoading, approvalBusyId, approvalError, toggleCompanyApproval }
}
