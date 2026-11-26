import { NextRequest } from 'next/server'
import { POST } from '@/app/api/auth/profile/route'

describe('POST /api/auth/profile', () => {
  afterEach(() => jest.restoreAllMocks())

  it('Backend が再発行したトークンは Cookie に入れ、本文には返さない（SOCAIAGENT-683）', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({ user_id: 7, name: '更新後', user_token: 'user-jwt', refresh_token: 'refresh' }),
        { status: 200 },
      ),
    )
    const response = await POST(
      new NextRequest('http://localhost:3000/api/auth/profile', {
        method: 'POST',
        headers: { 'content-type': 'application/json', origin: 'http://localhost:3000' },
        body: JSON.stringify({ name: '更新後' }),
      }),
    )
    const body = await response.json()
    expect(body).toEqual({ user_id: 7, name: '更新後' })
    expect(response.cookies.get('user_token')?.value).toBe('user-jwt')
  })
})
