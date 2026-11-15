/**
 * 企業一覧の判定ロジック（React 非依存の純関数）。
 */
import type { Company } from '../../types'

export type ResultsTab = 'all' | 'favorite' | 'applied'

export const RESULTS_TABS: { key: ResultsTab; label: string }[] = [
  { key: 'all', label: 'すべて' },
  { key: 'favorite', label: 'お気に入り' },
  { key: 'applied', label: '応募済み' },
]

export const TAB_PANEL_ID = 'results-tabpanel'
export const tabId = (tab: ResultsTab): string => `results-tab-${tab}`

/** タブの絞り込み条件。 */
export function matchesTab(company: Pick<Company, 'isFavorited' | 'isApplied'>, tab: ResultsTab): boolean {
  if (tab === 'favorite') return company.isFavorited === true
  if (tab === 'applied') return company.isApplied === true
  return true
}

/** タブごとの件数。 */
export function countByTab(companies: Pick<Company, 'isFavorited' | 'isApplied'>[]): Record<ResultsTab, number> {
  return {
    all: companies.length,
    favorite: companies.filter((c) => matchesTab(c, 'favorite')).length,
    applied: companies.filter((c) => matchesTab(c, 'applied')).length,
  }
}

/** 一覧の中で最も多い根拠軸数。未設定は 0 とみなす。 */
export function maxMatchedAxisCount(companies: Pick<Company, 'matchedAxisCount'>[]): number {
  return Math.max(0, ...companies.map((c) => c.matchedAxisCount ?? 0))
}

/**
 * 他の候補より根拠が薄い行か。暫定評価のときは全体が薄いので個別の印は出さない。
 */
export function isThinEvidence(
  company: Pick<Company, 'matchedAxisCount'>,
  maxCount: number,
  isProvisional: boolean,
): boolean {
  return !isProvisional && typeof company.matchedAxisCount === 'number' && company.matchedAxisCount < maxCount
}

/** 業種・勤務地・従業員を全角スペースでつなぐ。 */
export function buildMetaLine(company: Pick<Company, 'industry' | 'location' | 'employees'>): string {
  return [company.industry, company.location, company.employees].filter(Boolean).join('　')
}
