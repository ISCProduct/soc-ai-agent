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

/**
 * 企業関係データを Next.js プロキシ経由で取得する（DB: company_relations）。
 *
 * 失敗時は空配列を返さず {@link CompanyDataFetchError} を throw する。
 * 呼び出し側は try/catch で捕捉し、エラー UI を表示すること。
 *
 * @throws {CompanyDataFetchError} HTTP エラーまたはネットワーク失敗時
 */
export async function fetchCompanyRelations(): Promise<CapitalRelation[]> {
  try {
    const response = await fetch('/api/companies/relations', { cache: 'no-store' })
    if (!response.ok) {
      throw new CompanyDataFetchError('企業関係データの取得に失敗しました', response.status)
    }
    return response.json()
  } catch (error) {
    if (error instanceof CompanyDataFetchError) throw error
    throw new CompanyDataFetchError('企業関係データの取得中にエラーが発生しました')
  }
}

/**
 * 企業市場情報を Next.js プロキシ経由で取得する（DB: company_market_info）。
 *
 * 失敗時は空配列を返さず {@link CompanyDataFetchError} を throw する。
 * 呼び出し側は try/catch で捕捉し、エラー UI を表示すること。
 *
 * @throws {CompanyDataFetchError} HTTP エラーまたはネットワーク失敗時
 */
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
