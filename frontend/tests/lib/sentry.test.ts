import { scrubSentryEvent } from '@/lib/sentry'
import type { ErrorEvent } from '@sentry/nextjs'

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
