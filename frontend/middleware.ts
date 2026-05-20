import { NextRequest, NextResponse } from 'next/server'

export function middleware(request: NextRequest) {
  const userId = request.cookies.get('user_id')?.value
  const userToken = request.cookies.get('user_token')?.value

  const requestHeaders = new Headers(request.headers)

  if (userId && userToken) {
    // httpOnly CookieからX-User-*ヘッダーを注入（クライアント送信ヘッダーを上書き）
    requestHeaders.set('X-User-ID', userId)
    requestHeaders.set('X-User-Token', userToken)
  }

  return NextResponse.next({
    request: { headers: requestHeaders },
  })
}

export const config = {
  matcher: '/api/:path*',
}
