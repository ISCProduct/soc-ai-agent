import { isGoogleAuthUrl } from '@/lib/google-auth-url'

describe('isGoogleAuthUrl', () => {
  it.each<[string, unknown, boolean]>([
    ['Google の認可URL', 'https://accounts.google.com/o/oauth2/v2/auth?client_id=x&redirect_uri=y', true],
    ['http は不可', 'http://accounts.google.com/o/oauth2/v2/auth', false],
    ['別ホストは不可', 'https://example.com/o/oauth2/v2/auth', false],
    ['サブドメイン詐称は不可', 'https://accounts.google.com.example.com/', false],
    ['ユーザー情報部での詐称は不可', 'https://accounts.google.com@example.com/', false],
    ['javascript: は不可', 'javascript:alert(1)', false],
    ['相対URLは不可', '/o/oauth2/v2/auth', false],
    ['不正な文字列は不可', 'not a url', false],
    ['空文字は不可', '', false],
    ['文字列以外は不可', undefined, false],
  ])('%s', (_name, value, expected) => {
    expect(isGoogleAuthUrl(value)).toBe(expected)
  })
})
