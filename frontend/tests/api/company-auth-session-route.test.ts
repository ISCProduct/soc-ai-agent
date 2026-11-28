import { NextRequest } from 'next/server'
import { GET } from '@/app/api/company-auth/session/route'

describe('GET /api/company-auth/session', () => {
  it('トークンの値は返さない（SOCAIAGENT-683）', async () => {
    const response = await GET(
      new NextRequest('http://localhost:3000/api/company-auth/session', {
        headers: { 'X-Company-User-ID': '1', 'X-Company-User-Token': 'company-jwt' },
      }),
    )
    expect(await response.json()).toEqual({ company_user_id: 1 })
  })

  it('ログインしていなければ 401', async () => {
    const response = await GET(new NextRequest('http://localhost:3000/api/company-auth/session'))
    expect(response.status).toBe(401)
  })
})
