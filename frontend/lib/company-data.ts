export type MarketType = 'prime' | 'standard' | 'growth' | 'unlisted'
export type RelationType = 'subsidiary' | 'affiliate' // 子会社 or 関連会社

export interface Company {
  id: number
  name: string
  website_url?: string
  corporate_number?: string
  source_type?: string
  source_url?: string
  marketType?: MarketType
  isListed?: boolean
}

export interface CapitalRelation {
  id: number
  parent_id?: number
  child_id?: number
  from_id?: number
  to_id?: number
  relation_type: string
  ratio?: number
  description: string
  parent?: Company
  child?: Company
  from?: Company
  to?: Company
}

export interface CompanyMarketInfo {
  id: number
  company_id: number
  market_type: MarketType
  is_listed: boolean
  stock_code?: string
  company?: Company
}

export class CompanyDataFetchError extends Error {
  constructor(
    message: string,
    readonly status?: number,
  ) {
    super(message)
    this.name = 'CompanyDataFetchError'
  }
}

const demoCompanyNames = new Set([
  '株式会社テックイノベーション',
  'エンタープライズシステムズ株式会社',
  'クリエイティブラボ株式会社',
])

function isDemoCompany(company?: Company): boolean {
  if (!company) return true
  const website = company.website_url || ''
  if (website.includes('.example.com') || website === 'https://example.com') {
    return true
  }
  if (demoCompanyNames.has(company.name)) {
    return true
  }
  return false
}

function hasDemoEndpoint(relation: CapitalRelation): boolean {
  if (relation.parent_id && isDemoCompany(relation.parent)) return true
  if (relation.child_id && isDemoCompany(relation.child)) return true
  if (relation.from_id && isDemoCompany(relation.from)) return true
  if (relation.to_id && isDemoCompany(relation.to)) return true
  return false
}

/** 企業関係データを Next.js プロキシ経由で取得（DB: company_relations） */
export async function fetchCompanyRelations(): Promise<CapitalRelation[]> {
  try {
    const response = await fetch('/api/companies/relations', { cache: 'no-store' })
    if (!response.ok) {
      throw new CompanyDataFetchError('企業関係データの取得に失敗しました', response.status)
    }
    const relations: CapitalRelation[] = await response.json()
    return relations.filter((relation) => !hasDemoEndpoint(relation))
  } catch (error) {
    if (error instanceof CompanyDataFetchError) throw error
    throw new CompanyDataFetchError('企業関係データの取得中にエラーが発生しました')
  }
}

/** 企業市場情報を Next.js プロキシ経由で取得（DB: company_market_info） */
export async function fetchCompanyMarketInfo(): Promise<CompanyMarketInfo[]> {
  try {
    const response = await fetch('/api/companies/market-info', { cache: 'no-store' })
    if (!response.ok) {
      throw new CompanyDataFetchError('市場情報の取得に失敗しました', response.status)
    }
    return response.json()
  } catch (error) {
    if (error instanceof CompanyDataFetchError) throw error
    throw new CompanyDataFetchError('市場情報の取得中にエラーが発生しました')
  }
}

export const marketColors: Record<MarketType, string> = {
  prime: '#4169E1',
  standard: '#32CD32',
  growth: '#FF6347',
  unlisted: '#9E9E9E',
}

export const marketLabels: Record<MarketType, string> = {
  prime: 'プライム',
  standard: 'スタンダード',
  growth: 'グロース',
  unlisted: '非上場',
}
