/** @jest-environment jsdom */

import { companyStudentService } from '@/lib/company-students'

jest.mock('@/lib/company-auth', () => ({
  companyAuthService: {
    ensureFreshToken: jest.fn().mockResolvedValue(undefined),
    getAuthHeaders: () => ({ 'X-Company-User-Token': 'jwt-token' }),
  },
}))

function mockFetch(status: number, body: unknown) {
  const fetchMock = jest.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  })
  global.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

describe('companyStudentService', () => {
  afterEach(() => {
    jest.clearAllMocks()
  })

  it('フィルタをクエリ文字列へ変換する', async () => {
    const fetchMock = mockFetch(200, { items: [], total: 0 })

    await companyStudentService.search({
      industryId: 3,
      location: '東京',
      skill: '基本情報',
      tag: '候補A',
      limit: 20,
      offset: 40,
    })

    const url = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('/api/company-portal/students?')
    expect(url).toContain('industry_id=3')
    expect(url).toContain(`location=${encodeURIComponent('東京')}`)
    expect(url).toContain(`skill=${encodeURIComponent('基本情報')}`)
    expect(url).toContain(`tag=${encodeURIComponent('候補A')}`)
    expect(url).toContain('limit=20')
    expect(url).toContain('offset=40')
  })

  it('空のフィルタではクエリ文字列を付けない', async () => {
    const fetchMock = mockFetch(200, { items: [], total: 0 })

    await companyStudentService.search({})

    expect(fetchMock.mock.calls[0][0]).toBe('/api/company-portal/students')
  })

  it('company_id はクライアントから送らない（JWTで解決される）', async () => {
    const fetchMock = mockFetch(200, { items: [], total: 0 })

    await companyStudentService.search({ industryId: 3 })

    expect(fetchMock.mock.calls[0][0]).not.toContain('company_id')
  })

  it('企業トークンヘッダーを付与する', async () => {
    const fetchMock = mockFetch(200, { items: [], total: 0 })

    await companyStudentService.search({})

    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect((init.headers as Record<string, string>)['X-Company-User-Token']).toBe('jwt-token')
  })

  it('セマンティック検索はクエリをボディで送り、フィルタと併用できる', async () => {
    const fetchMock = mockFetch(200, { items: [], total: 0 })

    await companyStudentService.semanticSearch('リーダーシップ経験があってReactができる学生', {
      industryId: 3,
    })

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toContain('/api/company-portal/students/semantic-search?')
    expect(url).toContain('industry_id=3')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body as string)).toEqual({
      query: 'リーダーシップ経験があってReactができる学生',
    })
  })

  it('タグ追加はタグ名をボディで送る', async () => {
    const fetchMock = mockFetch(200, { tags: [] })

    await companyStudentService.addTag(5, '即戦力')

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/company-portal/students/5/tags')
    expect(JSON.parse(init.body as string)).toEqual({ tag_name: '即戦力' })
  })

  it('タグ削除は204でも例外にならない', async () => {
    mockFetch(204, null)
    await expect(companyStudentService.removeTag(5, 9)).resolves.toBeUndefined()
  })

  it('503はセマンティック検索の利用不可メッセージにする', async () => {
    mockFetch(503, null)
    await expect(companyStudentService.semanticSearch('React', {})).rejects.toThrow(
      /セマンティック検索を利用できません/,
    )
  })

  it('404は非公開の可能性を伝えるメッセージにする', async () => {
    mockFetch(404, null)
    await expect(companyStudentService.detail(5)).rejects.toThrow(/公開されていない/)
  })
})
