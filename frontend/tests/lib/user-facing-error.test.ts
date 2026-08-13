import {
  GATEWAY_USER_MESSAGE,
  gatewayErrorPath,
  looksLikeHtml,
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
})
