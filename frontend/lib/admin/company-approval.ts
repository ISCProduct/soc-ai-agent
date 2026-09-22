/**
 * 学校別の企業承認チップの表示状態を決める（#1452）。
 *
 * 承認状態の取得に失敗したときも空集合のままだったため、承認済みの企業まで
 * 一覧が全件「未承認」に見えていた。管理者には「この学校はまだ1社も承認して
 * いない」と映り、実態と区別がつかない。
 *
 * 「取得できていない」(null) を「承認されていない」と別の状態として扱う。
 */
export type ApprovalChipState = 'unknown' | 'approved' | 'unapproved'

/**
 * @param approvedCompanyIds 承認済み企業ID。null は取得できていないことを表す
 */
export function approvalChipState(
  approvedCompanyIds: Set<number> | null,
  companyId: number,
): ApprovalChipState {
  if (approvedCompanyIds === null) return 'unknown'
  return approvedCompanyIds.has(companyId) ? 'approved' : 'unapproved'
}

/** 承認/解除の操作を受け付けてよいか。取得できていない間は実態と食い違うため操作させない。 */
export function canToggleApproval(
  approvedCompanyIds: Set<number> | null,
  schoolId: number | undefined,
): boolean {
  return schoolId !== undefined && approvedCompanyIds !== null
}
