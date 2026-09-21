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

/** 相関図サイドパネル用の企業サマリー */
export interface CompanySummary {
  id: number
  name: string
  industry?: string
  location?: string
  description?: string
  main_business?: string
  employee_count?: number
  employee_count_basis?: 'consolidated' | 'standalone' | string
  founded_year?: number
  website_url?: string
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
 * プロキシが配列を `{ data: [...] }` にラップする場合と、生配列の両方を配列に正規化する。
 * @see parseProxyResponse / buildProxyJsonResponse
 */
export function unwrapCompanyListResponse<T>(raw: unknown): T[] {
  if (Array.isArray(raw)) return raw as T[]
  if (raw && typeof raw === 'object') {
    const data = (raw as { data?: unknown }).data
    if (Array.isArray(data)) return data as T[]
  }
  return []
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
    const raw: unknown = await response.json()
    return unwrapCompanyListResponse<CapitalRelation>(raw)
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
    const raw: unknown = await response.json()
    return unwrapCompanyListResponse<CompanyMarketInfo>(raw)
  } catch (error) {
    if (error instanceof CompanyDataFetchError) throw error
    throw new CompanyDataFetchError('市場情報の取得中にエラーが発生しました')
  }
}

export function formatEmployeeCount(
  count?: number | null,
  basis?: string | null,
): string {
  if (count == null || count <= 0) return ''
  const n = count.toLocaleString()
  if (basis === 'consolidated') return `${n}名（連結）`
  if (basis === 'standalone') return `${n}名（単体）`
  return `${n}名`
}

/**
 * 企業詳細（サマリー）を取得する。
 * @throws {CompanyDataFetchError} HTTP エラーまたはネットワーク失敗時
 */
export async function fetchCompanySummary(companyId: number): Promise<CompanySummary> {
  try {
    const response = await fetch(`/api/companies/${companyId}`, { cache: 'no-store' })
    if (!response.ok) {
      throw new CompanyDataFetchError('企業情報の取得に失敗しました', response.status)
    }
    const raw: unknown = await response.json()
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
      throw new CompanyDataFetchError('企業情報の形式が不正です')
    }
    const data = raw as Record<string, unknown>
    const id = typeof data.id === 'number' ? data.id : companyId
    const name = typeof data.name === 'string' ? data.name : `企業 ${companyId}`
    return {
      id,
      name,
      industry: typeof data.industry === 'string' ? data.industry : undefined,
      location: typeof data.location === 'string' ? data.location : undefined,
      description: typeof data.description === 'string' ? data.description : undefined,
      main_business: typeof data.main_business === 'string' ? data.main_business : undefined,
      employee_count: typeof data.employee_count === 'number' ? data.employee_count : undefined,
      employee_count_basis:
        data.employee_count_basis === 'consolidated' || data.employee_count_basis === 'standalone'
          ? data.employee_count_basis
          : undefined,
      founded_year: typeof data.founded_year === 'number' ? data.founded_year : undefined,
      website_url: typeof data.website_url === 'string' ? data.website_url : undefined,
    }
  } catch (error) {
    if (error instanceof CompanyDataFetchError) throw error
    throw new CompanyDataFetchError('企業情報の取得中にエラーが発生しました')
  }
}

export const marketColors: Record<MarketType, string> = {
  prime: '#3B6FD9',
  standard: '#2F9E44',
  growth: '#E8590C',
  unlisted: '#94A3B8',
}

export const marketLabels: Record<MarketType, string> = {
  prime: 'プライム',
  standard: 'スタンダード',
  growth: 'グロース',
  unlisted: '非上場',
}
