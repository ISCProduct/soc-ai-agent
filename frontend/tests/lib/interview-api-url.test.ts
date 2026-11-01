/**
 * interviewApi が組み立てる URL の検査。
 *
 * user_id クエリを外したとき(#1666)、クエリの開始文字が壊れて 400 になった。
 * クエリの開始文字は壊れても型では落ちないので URL を直接見る。
 * getDetail は role を送らない(#1814): 教員用レポートの開示はサーバーが実ロールで決める。
 */
import { interviewApi } from '@/lib/interview'

jest.mock('@/lib/auth/index', () => ({
  authService: {
    ensureFreshUserToken: jest.fn().mockResolvedValue(undefined),
    getUserFetchHeaders: () => ({ 'X-User-Token': 'jwt' }),
  },
}))

function okResponse(body: unknown = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('interviewApi の URL 組み立て', () => {
  let fetchMock: jest.Mock

  beforeEach(() => {
    fetchMock = jest.fn().mockResolvedValue(okResponse())
    global.fetch = fetchMock as unknown as typeof fetch
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  const calledUrl = () => String(fetchMock.mock.calls[0][0])

  it('getDetail は role をクエリに付けない（認可はサーバー側の実ロール。#1814）', async () => {
    await interviewApi.getDetail(5, 1)
    expect(calledUrl()).toMatch(/\/api\/interviews\/5$/)
    expect(calledUrl()).not.toContain('role')
  })

  it('user_id をクエリに付けない', async () => {
    await interviewApi.getDetail(5, 42)
    expect(calledUrl()).not.toContain('user_id')
  })

  it('listSessions は ?page= で始まる', async () => {
    await interviewApi.listSessions(1, 2, 10)
    expect(calledUrl()).toContain('?page=2')
    expect(calledUrl()).not.toContain('user_id')
  })

  it('getTrend は limit 指定時だけクエリを付け、?で始まる', async () => {
    await interviewApi.getTrend(1, 5)
    expect(calledUrl()).toContain('/api/interviews/trend?limit=5')
  })

  it('getTrend は limit 未指定ならクエリ無し', async () => {
    await interviewApi.getTrend(1)
    expect(calledUrl()).toMatch(/\/api\/interviews\/trend$/)
  })
})
