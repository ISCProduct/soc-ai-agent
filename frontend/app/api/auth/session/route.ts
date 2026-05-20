import { NextRequest, NextResponse } from 'next/server'

const COOKIE_OPTIONS = {
  httpOnly: true,
  secure: process.env.NODE_ENV === 'production',
  sameSite: 'strict' as const,
  path: '/',
  maxAge: 60 * 60 * 24 * 7, // 7日間
}

export async function POST(request: NextRequest) {
  const { userId, userToken } = await request.json()
  if (!userId || !userToken) {
    return NextResponse.json({ error: 'userId and userToken are required' }, { status: 400 })
  }

  const response = NextResponse.json({ ok: true })
  response.cookies.set('user_id', String(userId), COOKIE_OPTIONS)
  response.cookies.set('user_token', userToken, COOKIE_OPTIONS)
  return response
}

export async function DELETE() {
  const response = NextResponse.json({ ok: true })
  response.cookies.delete('user_id')
  response.cookies.delete('user_token')
  return response
}
