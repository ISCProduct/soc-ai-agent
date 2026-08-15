import { extractErrorMessage } from '@/lib/auth'

function jsonResponse(body: unknown): Response {
  return { json: async () => body } as Response
}

describe('extractErrorMessage', () => {
  it('既知の英語エラーを日本語に変換する', async () => {
    const res = jsonResponse({ error: 'invalid email or password' })
    expect(await extractErrorMessage(res, 'fallback')).toBe(
      'メールアドレスまたはパスワードが正しくありません。',
    )
  })

  it('未知のエラーメッセージはフォールバックを返す(英語の生文字列をそのまま出さない)', async () => {
    const res = jsonResponse({ error: 'some unmapped backend error' })
    expect(await extractErrorMessage(res, 'フォールバック文言')).toBe('フォールバック文言')
  })

  it('JSONでないレスポンスはフォールバックを返す', async () => {
    const res = { json: async () => { throw new Error('not json') } } as unknown as Response
    expect(await extractErrorMessage(res, 'フォールバック文言')).toBe('フォールバック文言')
  })
})
