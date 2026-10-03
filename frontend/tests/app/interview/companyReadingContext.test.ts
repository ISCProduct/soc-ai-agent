import { applyCompanyReadingResponse } from '@/app/interview/companyReadingContext'

describe('applyCompanyReadingResponse', () => {
  it('ignores a response from an earlier interview generation', () => {
    const current = { reading: 'いまの会社', readingResolved: true }
    const result = applyCompanyReadingResponse(
      current,
      { company_reading: '前の会社', company_reading_resolved: true },
      3,
      4,
    )

    expect(result).toBe(current)
  })

  it('keeps an empty reading resolved for the current interview', () => {
    const result = applyCompanyReadingResponse(
      { reading: '', readingResolved: false },
      { company_reading: '', company_reading_resolved: true },
      4,
      4,
    )

    expect(result).toEqual({ reading: '', readingResolved: true })
  })
})
