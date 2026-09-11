import {
  mapCompanyApiToViewModel,
  parseJsonArray,
  unwrapCompanyRecord,
} from '@/app/company/[id]/companyDetailUtils'

describe('parseJsonArray', () => {
  it('JSON 配列文字列をパースする', () => {
    expect(parseJsonArray('["Go","TypeScript"]')).toEqual(['Go', 'TypeScript'])
  })

  it('カンマ区切りを配列にする', () => {
    expect(parseJsonArray('Go, TypeScript')).toEqual(['Go', 'TypeScript'])
  })

  it('空・undefined は空配列', () => {
    expect(parseJsonArray()).toEqual([])
    expect(parseJsonArray('')).toEqual([])
  })
})

describe('unwrapCompanyRecord', () => {
  it('生オブジェクトをそのまま返す', () => {
    expect(unwrapCompanyRecord({ id: 1, name: 'A' })).toEqual({ id: 1, name: 'A' })
  })

  it('{ data: {...} } を展開する', () => {
    expect(unwrapCompanyRecord({ data: { id: 2, name: 'B' } })).toEqual({ id: 2, name: 'B' })
  })

  it('不正値は null', () => {
    expect(unwrapCompanyRecord(null)).toBeNull()
    expect(unwrapCompanyRecord([])).toBeNull()
  })
})

describe('mapCompanyApiToViewModel', () => {
  it('API フィールドを UI 向けに補完する', () => {
    const vm = mapCompanyApiToViewModel({
      id: 40045,
      name: 'テスト株式会社',
      industry: 'IT',
      location: '東京',
      description: '説明',
      employee_count: 100,
      founded_year: 2000,
      website_url: 'https://example.com',
      tech_stack: '["Go"]',
      culture: '風通しが良い',
    })

    expect(vm).toMatchObject({
      id: 40045,
      name: 'テスト株式会社',
      employees: '100名',
      size: '100名規模',
      founded: '2000年',
      website: 'https://example.com',
      techStack: ['Go'],
      culture: ['風通しが良い'],
    })
  })

  it('従業員数に連結/単体を付ける', () => {
    const vm = mapCompanyApiToViewModel({
      id: 1,
      name: 'NEC',
      employee_count: 101800,
      employee_count_basis: 'consolidated',
    })
    expect(vm?.employees).toBe('101,800名（連結）')
  })

  it('ラップされたレスポンスも扱える', () => {
    const vm = mapCompanyApiToViewModel({
      data: { id: 1, name: 'Wrapped' },
    })
    expect(vm?.name).toBe('Wrapped')
  })
})
