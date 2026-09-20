import { scrubSentryBreadcrumb, scrubSentryEvent } from '@/lib/sentry'
import type { Breadcrumb, ErrorEvent } from '@sentry/nextjs'

describe('scrubSentryEvent', () => {
  it('認証ヘッダーとボディを落とす', () => {
    const event = {
      request: {
        data: { resume_text: '秘密' },
        cookies: { session: 'abc' },
        query_string: 'email=a@b.com',
        headers: {
          Authorization: 'Bearer secret',
          Cookie: 'a=1',
          'X-Admin-Token': 'admin',
          'X-Request-ID': 'keep',
          'Content-Type': 'application/json',
        },
      },
    } as unknown as ErrorEvent

    const out = scrubSentryEvent(event)
    expect(out).not.toBeNull()
    expect(out?.request?.data).toBeUndefined()
    expect(out?.request?.cookies).toBeUndefined()
    expect(out?.request?.query_string).toBe('')
    expect(out?.request?.headers?.Authorization).toBeUndefined()
    expect(out?.request?.headers?.Cookie).toBeUndefined()
    expect(out?.request?.headers?.['X-Admin-Token']).toBeUndefined()
    expect(out?.request?.headers?.['X-Request-ID']).toBe('keep')
  })
})

// httpContextIntegration が window.location.href をそのまま request.url に入れるため、
// query_string を空にするだけでは足りない。クエリにトークンを載せる画面が実在する:
//   /verify-email?token= / /company-portal/setup?token= / /auth/callback?user=
describe('scrubSentryEvent の URL', () => {
  it('request.url のクエリを落とす', () => {
    const event = {
      request: { url: 'https://shukatsu-ai.jp/verify-email?token=ONETIME', query_string: 'token=ONETIME' },
    } as unknown as ErrorEvent

    const out = scrubSentryEvent(event)
    expect(out?.request?.url).toBe('https://shukatsu-ai.jp/verify-email')
    expect(out?.request?.url).not.toContain('ONETIME')
  })

  it('Referer も落とす（遷移元のクエリにトークンが載る）', () => {
    const event = {
      request: { headers: { Referer: 'https://shukatsu-ai.jp/auth/callback?user=BASE64' } },
    } as unknown as ErrorEvent

    expect(scrubSentryEvent(event)?.request?.headers?.Referer).toBeUndefined()
  })
})

// パンくずは beforeSend の対象外。fetch/navigation の URL をそのまま持つ。
describe('scrubSentryBreadcrumb', () => {
  it('fetch のURLからクエリを落とす', () => {
    const out = scrubSentryBreadcrumb({
      category: 'fetch',
      data: { url: '/api/auth/verify-email?token=ONETIME' },
    } as Breadcrumb)
    expect(out?.data?.url).toBe('/api/auth/verify-email')
  })

  it('画面遷移の from/to からクエリを落とす', () => {
    const out = scrubSentryBreadcrumb({
      category: 'navigation',
      data: { from: '/results?session=abc', to: '/verify-email?token=ONETIME' },
    } as Breadcrumb)
    expect(out?.data?.from).toBe('/results')
    expect(out?.data?.to).toBe('/verify-email')
  })

  it('console のパンくずは丸ごと捨てる（何が載るか読めない）', () => {
    expect(scrubSentryBreadcrumb({ category: 'console', message: '秘密' } as Breadcrumb)).toBeNull()
  })
})
