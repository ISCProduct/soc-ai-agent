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

const GENERIC_RELATION_LABELS = new Set([
  '子会社',
  '関連会社',
  '主要取引先',
  '取引先',
  '調達・契約',
  '補助金連携',
  'ビジネス関係',
  '資本関係',
  '関連',
  '調達（gBiz）',
  '補助金（gBiz）',
])

/** 取得元タグのプレースホルダ説明か（例: web_search:企業名） */
export function isSourceTagDescription(description: string): boolean {
  const trimmed = description.trim()
  if (!trimmed) return true
  const colon = trimmed.indexOf(':')
  if (colon <= 0) return false
  const tag = trimmed.slice(0, colon).trim().toLowerCase()
  return (SOURCE_TAG_PREFIXES as readonly string[]).includes(tag)
}

/** 関係種別ラベルだけで、取引内容ではないか */
export function isGenericRelationLabel(description: string): boolean {
  return GENERIC_RELATION_LABELS.has(description.trim())
}

/**
 * 編集フォーム／保存用: 実取引内容だけを残す。
 * 種別ラベルや取得元タグは空にする（「主要取引先」を説明欄に入れない）。
 */
export function sanitizeRelationDescription(description: string): string {
  const trimmed = description.trim()
  if (!trimmed || isSourceTagDescription(trimmed) || isGenericRelationLabel(trimmed)) {
    return ''
  }
  return trimmed
}

const RELATION_TYPE_LABELS_SHORT: Record<string, string> = {
  capital_subsidiary: '子会社',
  capital_affiliate: '関連',
  business_partner: '取引先',
  business_procurement: '調達',
  business_subsidy: '補助',
}

/** 図・一覧向けの関係ラベル（実内容優先、無ければ種別） */
export function formatRelationLabel(description: string, relationType: string): string {
  const real = sanitizeRelationDescription(description)
  if (real) return real
  if (RELATION_TYPE_LABELS[relationType]) {
    return RELATION_TYPE_LABELS[relationType]
  }
  if (relationType.startsWith('business_')) return 'ビジネス関係'
  if (relationType.startsWith('capital_')) return '資本関係'
  return relationType || '関連'
}

/**
 * 取引内容の表示用。具体内容が無ければフォールバックとして種別ラベル（取引先は「主要取引先」）。
 */
export function displayRelationDescription(description: string, relationType: string): string {
  return formatRelationLabel(description, relationType)
}

/** 具体的な取引内容が無く、種別フォールバック表示になるか */
export function isRelationDescriptionFallback(description: string): boolean {
  return sanitizeRelationDescription(description) === ''
}

/** 関係先として保存してよい、はっきりした組織名か */
export function isClearOrganizationName(name: string): boolean {
  const n = name.trim()
  if (!n) return false
  const unclearExact = new Set([
    '不明',
    '未定',
    'その他',
    'なし',
    '無し',
    '該当なし',
    'n/a',
    'na',
    '-',
    '—',
    '－',
    '複数社',
    '各社',
    '関係会社',
    'グループ会社',
    'グループ',
    '共同企業体',
    '特定共同企業体',
    'コンソーシアム',
    'jv',
    'ｊｖ',
    '株式会社',
    '有限会社',
    '合同会社',
    '主要取引先',
    '取引先',
  ])
  if (unclearExact.has(n) || unclearExact.has(n.toLowerCase())) return false
  if (/不明|未定|その他多数|ほか数社|他数社|等数社|共同企業体|特定共同企業体/.test(n)) {
    return false
  }
  const core = n
    .replace(/^(株式会社|有限会社|合同会社|合資会社|合名会社|一般社団法人|一般財団法人|公益社団法人|公益財団法人)\s*/u, '')
    .replace(/\s*(株式会社|有限会社|合同会社|合資会社|合名会社|Inc\.?|Ltd\.?|LLC|Corp\.?)$/iu, '')
    .trim()
  if (!core || unclearExact.has(core.toLowerCase())) return false
  if ([...core].length < 2) return false
  return true
}


/** 関連図エッジ向けの短いラベル（重なり防止） */
export function formatRelationLabelShort(description: string, relationType: string): string {
  const real = sanitizeRelationDescription(description)
  if (!real) {
    return RELATION_TYPE_LABELS_SHORT[relationType] || formatRelationLabel(description, relationType)
  }
  return real.length > 12 ? `${real.slice(0, 12)}…` : real
}
