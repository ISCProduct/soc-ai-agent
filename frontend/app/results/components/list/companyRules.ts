/**
 * 企業一覧の判定ロジック（React 非依存の純関数）。
 */
import type { Company } from '../../types'

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
