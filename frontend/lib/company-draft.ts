export type CompanyPublishState = {
  data_status?: string
  is_provisional?: boolean
}

export function isCompanyUnpublished(company?: CompanyPublishState | null): boolean {
  if (!company) return false
  if (company.is_provisional) return true
  return company.data_status != null && company.data_status !== 'published'
}
