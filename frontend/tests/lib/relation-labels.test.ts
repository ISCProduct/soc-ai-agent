import { formatRelationLabel, isSourceTagDescription } from '@/lib/relation-labels'

describe('relation-labels', () => {
  it('取得元タグのプレースホルダを検出する', () => {
    expect(isSourceTagDescription('web_search:sky株式会社')).toBe(true)
    expect(isSourceTagDescription('llm_web_search:テスト')).toBe(true)
    expect(isSourceTagDescription('決済代行')).toBe(false)
  })

  it('関係タイプから表示ラベルを返す', () => {
    expect(formatRelationLabel('web_search:sky株式会社', 'business_partner')).toBe('主要取引先')
    expect(formatRelationLabel('', 'business_procurement')).toBe('調達・契約')
    expect(formatRelationLabel('クラウド基盤共同開発', 'business_partner')).toBe('クラウド基盤共同開発')
  })
})
