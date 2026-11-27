import {
  GATEWAY_USER_MESSAGE,
  gatewayErrorPath,
  looksLikeHtml,
  safeReturnPath,
  userFacingApiMessage,
} from '@/lib/user-facing-error'

const NGINX_502 = `<html>
<head><title>502 Bad Gateway</title></head>
<body>
<center><h1>502 Bad Gateway</h1></center>
</body>
</html>`

describe('user-facing-error', () => {
  it('nginx の 502 HTML を検知する', () => {
    expect(looksLikeHtml(NGINX_502)).toBe(true)
    expect(looksLikeHtml('{"error":"nope"}')).toBe(false)
  })

  it('502 / HTML はユーザー向け文言だけ返す', () => {
    expect(userFacingApiMessage(502, NGINX_502)).toBe(GATEWAY_USER_MESSAGE)
    expect(userFacingApiMessage(502, NGINX_502)).not.toMatch(/<html|Bad Gateway|Chat API/)
    expect(gatewayErrorPath(502)).toBe('/error?code=502')
  })

  // #2213: エラー画面の「再試行」で、エラーになったページへ戻る
  it('戻り先を渡すと from に入れる', () => {
    expect(gatewayErrorPath(503, '/chat?session=1')).toBe('/error?code=503&from=%2Fchat%3Fsession%3D1')
    expect(gatewayErrorPath(500, '/chat')).toBe('/error?code=502&from=%2Fchat')
  })

  it('外のサイトやエラー画面そのものは戻り先にしない', () => {
    expect(gatewayErrorPath(502, 'https://evil.example')).toBe('/error?code=502')
    for (const bad of [
      'https://evil.example',
      '//evil.example',
      '/\\evil.example',
      'javascript:alert(1)',
      'chat',
      '',
      '/error',
      '/error?code=502',
      '/chat\nSet-Cookie: x',
      '/' + 'a'.repeat(3000),
    ]) {
      expect(safeReturnPath(bad)).toBeNull()
    }
    expect(safeReturnPath(null)).toBeNull()
    expect(safeReturnPath(undefined)).toBeNull()
  })

  it('同じサイトの中のパスは戻り先にする', () => {
    expect(safeReturnPath('/chat')).toBe('/chat')
    expect(safeReturnPath('/results?session=3#top')).toBe('/results?session=3#top')
    expect(safeReturnPath('/errors-help')).toBe('/errors-help')
  })
})
