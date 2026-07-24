import {
  unwrapCompanyListResponse,
  fetchCompanyRelations,
  fetchCompanyMarketInfo,
  CompanyDataFetchError,
} from '@/lib/company-data'

describe('unwrapCompanyListResponse', () => {
  it('returns a raw array as-is', () => {
    const list = [{ id: 1 }]
    expect(unwrapCompanyListResponse(list)).toBe(list)
  })

  it('unwraps { data: [...] } from buildProxyJsonResponse', () => {
    const list = [{ id: 2, relation_type: 'capital_subsidiary' }]
    expect(unwrapCompanyListResponse({ data: list })).toEqual(list)
  })

  it('returns [] for null, object without data array, or non-array data', () => {
    expect(unwrapCompanyListResponse(null)).toEqual([])
    expect(unwrapCompanyListResponse(undefined)).toEqual([])
    expect(unwrapCompanyListResponse({})).toEqual([])
    expect(unwrapCompanyListResponse({ data: 'x' })).toEqual([])
    expect(unwrapCompanyListResponse({ error: 'fail' })).toEqual([])
  })
})

describe('fetchCompanyRelations', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
  })

  it('unwraps proxied array payload', async () => {
    const relations = [{ id: 1, relation_type: 'capital_subsidiary', description: '' }]
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: relations }),
    }) as unknown as typeof fetch

    await expect(fetchCompanyRelations()).resolves.toEqual(relations)
  })

  it('throws CompanyDataFetchError on HTTP failure', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 500,
    }) as unknown as typeof fetch

    await expect(fetchCompanyRelations()).rejects.toBeInstanceOf(CompanyDataFetchError)
  })
})

describe('fetchCompanyMarketInfo', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
  })

  it('unwraps proxied array payload', async () => {
    const market = [{ id: 1, company_id: 9, market_type: 'prime', is_listed: true }]
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ data: market }),
    }) as unknown as typeof fetch

    await expect(fetchCompanyMarketInfo()).resolves.toEqual(market)
  })
})
