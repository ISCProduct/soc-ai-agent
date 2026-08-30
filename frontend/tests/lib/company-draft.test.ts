import { isCompanyUnpublished } from '@/lib/company-draft'

describe('isCompanyUnpublished', () => {
  it('published なら警告しない', () => {
    expect(isCompanyUnpublished({ data_status: 'published' })).toBe(false)
  })

  it('draft や仮登録なら警告する', () => {
    expect(isCompanyUnpublished({ data_status: 'draft' })).toBe(true)
    expect(isCompanyUnpublished({ is_provisional: true, data_status: 'published' })).toBe(true)
  })
})
