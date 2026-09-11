import {
  displayRelationDescription,
  formatRelationLabel,
  isClearOrganizationName,
  isGenericRelationLabel,
  isRelationDescriptionFallback,
  isSourceTagDescription,
  sanitizeRelationDescription,
} from '@/lib/relation-labels'

describe('relation-labels', () => {
  it('取得元タグのプレースホルダを検出する', () => {
    expect(isSourceTagDescription('web_search:sky株式会社')).toBe(true)
    expect(isSourceTagDescription('llm_web_search:テスト')).toBe(true)
    expect(isSourceTagDescription('決済代行')).toBe(false)
  })

  it('種別ラベルだけの説明を検出する', () => {
    expect(isGenericRelationLabel('主要取引先')).toBe(true)
    expect(isGenericRelationLabel('取引先')).toBe(true)
    expect(isGenericRelationLabel('決済代行')).toBe(false)
  })

  it('編集用に種別ラベルを落とす', () => {
    expect(sanitizeRelationDescription('主要取引先')).toBe('')
    expect(sanitizeRelationDescription('web_search:sky株式会社')).toBe('')
    expect(sanitizeRelationDescription('決済代行')).toBe('決済代行')
  })

  it('図表示では実内容を優先し、無ければ種別ラベル', () => {
    expect(formatRelationLabel('web_search:sky株式会社', 'business_partner')).toBe('主要取引先')
    expect(formatRelationLabel('主要取引先', 'business_partner')).toBe('主要取引先')
    expect(formatRelationLabel('', 'business_procurement')).toBe('調達・契約')
    expect(formatRelationLabel('クラウド基盤共同開発', 'business_partner')).toBe('クラウド基盤共同開発')
  })

  it('弱いフォールバックは主要取引先表示', () => {
    expect(isRelationDescriptionFallback('')).toBe(true)
    expect(isRelationDescriptionFallback('主要取引先')).toBe(true)
    expect(isRelationDescriptionFallback('決済代行')).toBe(false)
    expect(displayRelationDescription('', 'business_partner')).toBe('主要取引先')
    expect(displayRelationDescription('決済代行', 'business_partner')).toBe('決済代行')
  })

  it('曖昧な組織名を弾く', () => {
    expect(isClearOrganizationName('デジタル庁')).toBe(true)
    expect(isClearOrganizationName('株式会社パートナーA')).toBe(true)
    expect(isClearOrganizationName('共同企業体')).toBe(false)
    expect(isClearOrganizationName('その他')).toBe(false)
    expect(isClearOrganizationName('株式会社')).toBe(false)
    expect(isClearOrganizationName('')).toBe(false)
  })
})
