import { BACKEND_URL } from './config';

// 企業データの型定義
export type MarketType = 'prime' | 'standard' | 'growth' | 'unlisted';
export type RelationType = 'subsidiary' | 'affiliate'; // 子会社 or 関連会社

export interface Company {
  id: number;
  name: string;
  website_url?: string;
  corporate_number?: string;
  source_type?: string;
  source_url?: string;
  marketType?: MarketType;
  isListed?: boolean;
}

export interface CapitalRelation {
  id: number;
  parent_id?: number;
  child_id?: number;
  from_id?: number;
  to_id?: number;
  relation_type: string;
  ratio?: number;
  description: string;
  parent?: Company;
  child?: Company;
  from?: Company;
  to?: Company;
}

export interface CompanyMarketInfo {
  id: number;
  company_id: number;
  market_type: MarketType;
  is_listed: boolean;
  stock_code?: string;
  company?: Company;
}

export class CompanyDataFetchError extends Error {
  readonly status?: number;

  constructor(message: string, status?: number) {
    super(message);
    this.name = 'CompanyDataFetchError';
    this.status = status;
  }
}

async function fetchJson<T>(url: string, resourceLabel: string): Promise<T> {
  try {
    const response = await fetch(url);
    if (!response.ok) {
      throw new CompanyDataFetchError(`${resourceLabel}の取得に失敗しました`, response.status);
    }
    return response.json();
  } catch (error) {
    if (error instanceof CompanyDataFetchError) {
      throw error;
    }
    throw new CompanyDataFetchError(`${resourceLabel}の取得中にエラーが発生しました`);
  }
}

// APIから企業関係データを取得
export async function fetchCompanyRelations(): Promise<CapitalRelation[]> {
  return fetchJson<CapitalRelation[]>(`${BACKEND_URL}/api/companies/relations`, '企業関係データ');
}

// APIから企業市場情報を取得
export async function fetchCompanyMarketInfo(): Promise<CompanyMarketInfo[]> {
  return fetchJson<CompanyMarketInfo[]>(`${BACKEND_URL}/api/companies/market-info`, '市場情報');
}

// 市場区分の色定義
export const marketColors: Record<MarketType, string> = {
  prime: '#4169E1',      // プライム：ロイヤルブルー
  standard: '#32CD32',   // スタンダード：ライムグリーン
  growth: '#FF6347',     // グロース：トマトレッド
  unlisted: '#9E9E9E',   // 非上場：グレー
};

export const marketLabels: Record<MarketType, string> = {
  prime: 'プライム',
  standard: 'スタンダード',
  growth: 'グロース',
  unlisted: '非上場',
};
