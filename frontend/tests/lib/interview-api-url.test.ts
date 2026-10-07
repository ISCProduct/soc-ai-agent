/**
 * interviewApi が組み立てる URL の検査。
 *
 * user_id クエリを外したとき(#1666)、`getDetail` の roleParam が `&role=` のまま
 * 残り `/api/interviews/5&role=teacher` を要求していた。role がクエリにならず
 * id の一部として読まれるため UintParam が 400 を返し、履歴から教員が面接レポートを
 * 開けなくなっていた。クエリの開始文字は壊れても型では落ちないので URL を直接見る。
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

  it('role を渡すと ?role= で始まるクエリになる', async () => {
    await interviewApi.getDetail(5, 1, 'teacher')
    expect(calledUrl()).toContain('/api/interviews/5?role=teacher')
    // `&role=` だと id の一部として読まれて 400 になる。
    expect(calledUrl()).not.toContain('5&role=')
  })

  it('role を渡さなければクエリを付けない', async () => {
    await interviewApi.getDetail(5, 1)
    expect(calledUrl()).toMatch(/\/api\/interviews\/5$/)
  })

  it('role はエスケープされる', async () => {
    await interviewApi.getDetail(5, 1, 'a&b=c')
    expect(calledUrl()).toContain('?role=a%26b%3Dc')
  })

  it('user_id をクエリに付けない', async () => {
    await interviewApi.getDetail(5, 42, 'teacher')
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
