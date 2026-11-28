import { NextRequest } from 'next/server'
import { POST } from '@/app/api/user/consultations/route'
import { askTeacher } from '@/lib/consultation'

// #2028: 学生向けの理由（学校が未登録など）は、プロキシを通しても学生にそのまま見せる。

jest.mock('@/lib/auth', () => ({
  authService: {
    ensureFreshUserToken: jest.fn().mockResolvedValue(undefined),
    getUserFetchHeaders: () => ({ 'X-User-Token': 't' }),
  },
}))

const backend = (status: number, body: unknown) =>
  new Response(typeof body === 'string' ? body : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

const post = () =>
  POST(new NextRequest('http://localhost:3000/api/user/consultations', {
    method: 'POST',
    headers: { 'X-User-Token': 't', 'Content-Type': 'application/json' },
    body: JSON.stringify({ body: '質問' }),
  }))

describe('POST /api/user/consultations のエラー', () => {
  afterEach(() => jest.restoreAllMocks())

  it.each([400, 404, 429])('%i は Backend の error をそのまま返す', async (status) => {
    jest.spyOn(global, 'fetch').mockResolvedValue(backend(status, { error: '学校が登録されていないため、先生に質問を送れません。', code: 'BAD_REQUEST' }))
    const res = await post()
    expect(res.status).toBe(status)
    expect(await res.json()).toEqual({ error: '学校が登録されていないため、先生に質問を送れません。', status })
  })

  it('error が読めなければ一般的な文言にする', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(backend(400, 'not json'))
    const res = await post()
    expect(res.status).toBe(400)
    expect((await res.json()).error).toBe('処理に失敗しました。しばらくしてから再試行してください。')
  })

  it('500 は内部の文言を見せない（共通の処理）', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(backend(500, { error: 'sql: connection refused' }))
    const res = await post()
    expect(res.status).toBe(500)
    expect((await res.json()).error).not.toContain('sql')
  })

  it('学校が未登録の 400 の理由が、プロキシを通した形のまま askTeacher のエラーになる', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(backend(400, { error: '学校が登録されていないため、先生に質問を送れません。プロフィールで学校を設定してください', code: 'BAD_REQUEST' }))
    const proxied = await post()
    jest.restoreAllMocks()
    jest.spyOn(global, 'fetch').mockResolvedValue(proxied as unknown as Response)
    await expect(askTeacher('質問')).rejects.toThrow('プロフィールで学校を設定してください')
  })
})
