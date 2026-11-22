// 企業一覧を操作したあとに、どの GET を取り直すか。
// 業界一覧は is_active な企業の業界名（ListActiveIndustries）なので、
// 公開・審査では変わらず、非公開（is_active を落とす）と情報取得では変わりうる。

export type CompanyListReloadKind = 'status' | 'reject' | 'catalog'

export type CompanyListReloadTasks = {
  companies: () => Promise<void>
  industries: () => Promise<void>
  coverage: () => Promise<void>
}

export function companyListReloadRequests(kind: CompanyListReloadKind): {
  industries: boolean
  coverage: boolean
} {
  if (kind === 'catalog') return { industries: true, coverage: true }
  if (kind === 'reject') return { industries: true, coverage: false }
  return { industries: false, coverage: false }
}

export async function runCompanyListReload(
  kind: CompanyListReloadKind,
  tasks: CompanyListReloadTasks,
): Promise<void> {
  const plan = companyListReloadRequests(kind)
  const pending: Promise<void>[] = [tasks.companies()]
  if (plan.industries) pending.push(tasks.industries())
  if (plan.coverage) pending.push(tasks.coverage())
  await Promise.all(pending)
}
