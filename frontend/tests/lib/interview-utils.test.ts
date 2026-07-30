import { parseMediaError } from '@/lib/interview-utils'

describe('parseMediaError', () => {
  it('maps session Unauthorized to re-login guidance', () => {
    expect(parseMediaError(new Error('Unauthorized'))).toBe(
      'ログインの有効期限が切れました。再ログインしてから面接を開始してください。',
    )
  })

  it('maps OpenAI chat 401 to API key guidance', () => {
    expect(parseMediaError(new Error('chat error 401: incorrect API key'))).toBe(
      'AIサービスへの接続に失敗しました。（OpenAI APIキーを確認してください）',
    )
  })

  it('maps media permission errors', () => {
    expect(parseMediaError(new Error('NotAllowedError'))).toContain('マイクとカメラ')
  })
})
