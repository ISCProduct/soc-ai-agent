import { NextRequest } from 'next/server'
import { clientIpHeaders, extractUserAuthHeaders } from '@/lib/api-proxy'
import { POST as companyLogin } from '@/app/api/company-auth/login/route'

/**
 * BFF が Backend へ実クライアントIPを引き継ぐ経路の検証 (#1407)。
 *
 * 信頼境界: 実IPの出所は ALB が付けた X-Forwarded-For の末尾だけ。
 * クライアントが送ってきた X-Client-IP を読んではならない（読むと詐称できる）。
 */
function requestWith(headers: Record<string, string>): NextRequest {
  return new NextRequest('http://localhost:3000/api/company-auth/login', {
    method: 'POST',
    headers,
    body: JSON.stringify({ email: 'hr@example.com', password: 'pw' }),
  })
}

describe('clientIpHeaders', () => {
  const originalToken = process.env.BFF_INTERNAL_TOKEN
  const originalHops = process.env.TRUSTED_PROXY_HOPS

  afterEach(() => {
    if (originalToken === undefined) {
      delete process.env.BFF_INTERNAL_TOKEN
    } else {
      process.env.BFF_INTERNAL_TOKEN = originalToken
    }
    if (originalHops === undefined) {
      delete process.env.TRUSTED_PROXY_HOPS
    } else {
      process.env.TRUSTED_PROXY_HOPS = originalHops
    }
  })

  const cases: {
    name: string
    token: string | undefined
    hops?: string
    headers: Record<string, string>
    expected: Record<string, string>
  }[] = [
    {
      name: 'トークン未設定なら何も送らない（従来どおりBFFの出口IPで集計）',
      token: undefined,
      headers: { 'x-forwarded-for': '198.51.100.7' },
      expected: {},
    },
    {
      name: 'トークンが空白のみなら何も送らない',
      token: '   ',
      headers: { 'x-forwarded-for': '198.51.100.7' },
      expected: {},
    },
    {
      name: 'XFF が無ければ何も送らない（ローカル開発など）',
      token: 'secret-token',
      headers: {},
      expected: {},
    },
    {
      name: 'XFF 単一ならそのIPを送る',
      token: 'secret-token',
      headers: { 'x-forwarded-for': '198.51.100.7' },
      expected: { 'X-Client-IP': '198.51.100.7', 'X-Internal-Token': 'secret-token' },
    },
    {
      name: 'XFF が多段連結なら末尾（ALBが付けた実IP）を採る',
      token: 'secret-token',
      headers: { 'x-forwarded-for': '1.1.1.1, 2.2.2.2,  198.51.100.7 ' },
      expected: { 'X-Client-IP': '198.51.100.7', 'X-Internal-Token': 'secret-token' },
    },
    {
      name: 'クライアントが送ったX-Client-IPは読まない（詐称防止）',
      token: 'secret-token',
      headers: { 'x-forwarded-for': '198.51.100.7', 'x-client-ip': '203.0.113.99' },
      expected: { 'X-Client-IP': '198.51.100.7', 'X-Internal-Token': 'secret-token' },
    },
    {
      name: 'クライアントが送ったX-Internal-Tokenも読まない（環境変数の値のみ使う）',
      token: 'secret-token',
      headers: { 'x-forwarded-for': '198.51.100.7', 'x-internal-token': 'attacker-token' },
      expected: { 'X-Client-IP': '198.51.100.7', 'X-Internal-Token': 'secret-token' },
    },
    {
      name: 'CloudFront+ALB(2段)なら末尾から2番目を採る',
      token: 'secret-token',
      hops: '2',
      headers: { 'x-forwarded-for': '198.51.100.7, 203.0.113.10' },
      expected: { 'X-Client-IP': '198.51.100.7', 'X-Internal-Token': 'secret-token' },
    },
    {
      name: 'CloudFront+ALB+nginx(3段)なら末尾から3番目を採る',
      token: 'secret-token',
      hops: '3',
      headers: { 'x-forwarded-for': '1.1.1.1, 198.51.100.7, 203.0.113.10, 10.0.0.9' },
      expected: { 'X-Client-IP': '198.51.100.7', 'X-Internal-Token': 'secret-token' },
    },
    {
      name: '段数が要素数を超える(経路が想定と違う)なら転送しない',
      token: 'secret-token',
      hops: '3',
      headers: { 'x-forwarded-for': '198.51.100.7, 203.0.113.10' },
      expected: {},
    },
    {
      name: '段数が不正なら既定の1段として扱う',
      token: 'secret-token',
      hops: 'abc',
      headers: { 'x-forwarded-for': '1.1.1.1, 198.51.100.7' },
      expected: { 'X-Client-IP': '198.51.100.7', 'X-Internal-Token': 'secret-token' },
    },
    {
      name: '段数0以下も既定の1段として扱う',
      token: 'secret-token',
      hops: '0',
      headers: { 'x-forwarded-for': '1.1.1.1, 198.51.100.7' },
      expected: { 'X-Client-IP': '198.51.100.7', 'X-Internal-Token': 'secret-token' },
    },
  ]

  it.each(cases)('$name', ({ token, hops, headers, expected }) => {
    if (token === undefined) {
      delete process.env.BFF_INTERNAL_TOKEN
    } else {
      process.env.BFF_INTERNAL_TOKEN = token
    }
    if (hops === undefined) {
      delete process.env.TRUSTED_PROXY_HOPS
    } else {
      process.env.TRUSTED_PROXY_HOPS = hops
    }
    expect(clientIpHeaders(requestWith(headers))).toEqual(expected)
  })

  it('extractUserAuthHeaders でも同じヘッダーが付く（既存プロキシ経路の取りこぼし防止）', () => {
    process.env.BFF_INTERNAL_TOKEN = 'secret-token'
    const headers = extractUserAuthHeaders(
      requestWith({ 'x-forwarded-for': '1.1.1.1, 198.51.100.7', 'X-User-ID': '42' }),
    )
    expect(headers).toMatchObject({
      'X-Client-IP': '198.51.100.7',
      'X-Internal-Token': 'secret-token',
      'X-User-ID': '42',
    })
  })
})

describe('POST /api/company-auth/login', () => {
  const originalToken = process.env.BFF_INTERNAL_TOKEN

  afterEach(() => {
    jest.restoreAllMocks()
    if (originalToken === undefined) {
      delete process.env.BFF_INTERNAL_TOKEN
    } else {
      process.env.BFF_INTERNAL_TOKEN = originalToken
    }
  })

  it('実クライアントIPをBackendへ転送する（IP単位の制限が全企業共通にならない）', async () => {
    process.env.BFF_INTERNAL_TOKEN = 'secret-token'
    const fetchMock = jest
      .spyOn(global, 'fetch')
      .mockResolvedValue(new Response('{}', { status: 200 }))

    await companyLogin(requestWith({ 'x-forwarded-for': '1.1.1.1, 198.51.100.7' }))

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/company-auth\/login$/),
      expect.objectContaining({
        headers: expect.objectContaining({
          'X-Client-IP': '198.51.100.7',
          'X-Internal-Token': 'secret-token',
        }),
      }),
    )
  })

  it('トークン未設定なら転送しない（シークレット未配布でもログインは通る）', async () => {
    delete process.env.BFF_INTERNAL_TOKEN
    const fetchMock = jest
      .spyOn(global, 'fetch')
      .mockResolvedValue(new Response('{}', { status: 200 }))

    const response = await companyLogin(requestWith({ 'x-forwarded-for': '198.51.100.7' }))

    expect(response.status).toBe(200)
    const init = fetchMock.mock.calls[0][1] as RequestInit
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' })
  })
})
