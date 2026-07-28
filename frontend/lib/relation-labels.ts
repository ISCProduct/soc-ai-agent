const SOURCE_TAG_PREFIXES = [
  'web_search',
  'llm_web_search',
  'gbizinfo',
  'scrape',
  'llm_extract',
  'official',
  'manual',
  'job_site',
  'scraping',
] as const

const RELATION_TYPE_LABELS: Record<string, string> = {
  capital_subsidiary: '子会社',
  capital_affiliate: '関連会社',
  business_partner: '主要取引先',
  business_procurement: '調達・契約',
  business_subsidy: '補助金連携',
}

/** 取得元タグのプレースホルダ説明か（例: web_search:企業名） */
export function isSourceTagDescription(description: string): boolean {
  const trimmed = description.trim()
  if (!trimmed) return true
  const colon = trimmed.indexOf(':')
  if (colon <= 0) return false
  const tag = trimmed.slice(0, colon).trim().toLowerCase()
  return (SOURCE_TAG_PREFIXES as readonly string[]).includes(tag)
}

const RELATION_TYPE_LABELS_SHORT: Record<string, string> = {
  capital_subsidiary: '子会社',
  capital_affiliate: '関連',
  business_partner: '取引先',
  business_procurement: '調達',
  business_subsidy: '補助',
}

/** 図・一覧向けの関係ラベル */
export function formatRelationLabel(description: string, relationType: string): string {
  const trimmed = description.trim()
  if (trimmed && !isSourceTagDescription(trimmed)) {
    return trimmed
  }
  if (RELATION_TYPE_LABELS[relationType]) {
    return RELATION_TYPE_LABELS[relationType]
  }
  if (relationType.startsWith('business_')) return 'ビジネス関係'
  if (relationType.startsWith('capital_')) return '資本関係'
  return relationType || '関連'
}

/** 関連図エッジ向けの短いラベル（重なり防止） */
export function formatRelationLabelShort(description: string, relationType: string): string {
  const trimmed = description.trim()
  if (!trimmed || isSourceTagDescription(trimmed)) {
    return RELATION_TYPE_LABELS_SHORT[relationType] || formatRelationLabel(description, relationType)
  }
  return trimmed.length > 12 ? `${trimmed.slice(0, 12)}…` : trimmed
}
