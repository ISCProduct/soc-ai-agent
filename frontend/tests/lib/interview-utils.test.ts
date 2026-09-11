import { extractApiErrorMessage, parseMediaError } from '@/lib/interview-utils'

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

describe('extractApiErrorMessage', () => {
  it('extracts the error field from backend JSON error responses', () => {
    expect(extractApiErrorMessage('{"error":"内部エラーが発生しました","code":"INTERNAL_ERROR"}')).toBe(
      '内部エラーが発生しました',
    )
  })

  it('falls back to the raw body when it is not the expected JSON shape', () => {
    expect(extractApiErrorMessage('transcribe error: context deadline exceeded')).toBe(
      'transcribe error: context deadline exceeded',
    )
  })

  it('falls back to the raw body for JSON without an error field', () => {
    expect(extractApiErrorMessage('{"code":"INTERNAL_ERROR"}')).toBe('{"code":"INTERNAL_ERROR"}')
  })
})
