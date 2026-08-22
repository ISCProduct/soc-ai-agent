import { maxDuration as missingMaxDuration } from '@/app/api/admin/companies/fetch-missing-batch/route'
import { maxDuration as warmMaxDuration } from '@/app/api/admin/companies/warm-l1/route'

describe('admin company batch routes', () => {
  it('warm-l1 も不足取得と同じ15分まで待つ', () => {
    expect(warmMaxDuration).toBe(900)
    expect(missingMaxDuration).toBe(900)
  })
})
