import { adminFetchJson, toAdminErrorMessage } from '@/lib/admin-fetch'
import { FetchTimeoutError } from '@/lib/fetch-timeout'
import { GATEWAY_USER_MESSAGE, UserFacingApiError } from '@/lib/user-facing-error'

const originalFetch = global.fetch

/** fetchWithTimeout が呼ぶ fetch を差し替える */
function mockResponse(status: number, body: string) {
  global.fetch = jest.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    text: async () => body,
  })
}

describe('adminFetchJson (#1066)', () => {
  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('成功時はパース済みのJSONを返す', async () => {
    mockResponse(200, JSON.stringify({ users: [{ id: 1 }], total: 1 }))

    const data = await adminFetchJson<{ users: { id: number }[]; total: number }>(
      '/api/admin/users',
      undefined,
      '取得に失敗しました',
    )

    expect(data.total).toBe(1)
    expect(data.users[0].id).toBe(1)
  })

  it('204のように本文が空でも例外にせずundefinedを返す', async () => {
    mockResponse(204, '')

    await expect(adminFetchJson('/api/admin/users/1', { method: 'DELETE' }, '削除に失敗しました'))
      .resolves.toBeUndefined()
  })

  it('通信自体が失敗した場合はrejectする', async () => {
    global.fetch = jest.fn().mockRejectedValue(new TypeError('Failed to fetch'))

    await expect(adminFetchJson('/api/admin/users', undefined, '取得に失敗しました')).rejects.toThrow()
  })

  it('サーバーがerror文言を返した場合はそれを優先する', async () => {
    mockResponse(400, JSON.stringify({ error: '自分自身の権限は変更できません' }))

    await expect(
      adminFetchJson('/api/admin/users/1', { method: 'PUT' }, '権限更新に失敗しました'),
    ).rejects.toThrow('自分自身の権限は変更できません')
  })

  it('サーバーがerror文言を返さない場合はfallbackMessageを出す', async () => {
    mockResponse(400, '')

    await expect(
      adminFetchJson('/api/admin/users/1', { method: 'PUT' }, '権限更新に失敗しました'),
    ).rejects.toThrow('権限更新に失敗しました')
  })

  it('502はゲートウェイ文言にする（fallbackMessageより優先）', async () => {
    mockResponse(502, '')

    await expect(
      adminFetchJson('/api/admin/users', undefined, 'ユーザー一覧の取得に失敗しました'),
    ).rejects.toThrow(GATEWAY_USER_MESSAGE)
  })

  it('ALBのHTMLエラーページを画面に出さない', async () => {
    mockResponse(503, '<html><head><title>503 Service Temporarily Unavailable</title></head></html>')

    await expect(
      adminFetchJson('/api/admin/users', undefined, 'ユーザー一覧の取得に失敗しました'),
    ).rejects.toThrow(GATEWAY_USER_MESSAGE)
  })

  it('本文がHTMLならステータスが5xx以外でもゲートウェイ文言にする', async () => {
    mockResponse(404, '<!doctype html><html><body>Not Found</body></html>')

    await expect(
      adminFetchJson('/api/admin/users', undefined, 'ユーザー一覧の取得に失敗しました'),
    ).rejects.toThrow(GATEWAY_USER_MESSAGE)
  })

  it('失敗時はstatusを持つUserFacingApiErrorを投げる', async () => {
    mockResponse(403, JSON.stringify({ error: '権限がありません' }))

    await expect(
      adminFetchJson('/api/admin/users/1', { method: 'DELETE' }, '削除に失敗しました'),
    ).rejects.toBeInstanceOf(UserFacingApiError)
  })
})

describe('toAdminErrorMessage (#1066)', () => {
  it('UserFacingApiErrorはメッセージをそのまま使う', () => {
    expect(toAdminErrorMessage(new UserFacingApiError('権限更新に失敗しました', 400)))
      .toBe('権限更新に失敗しました')
  })

  it('FetchTimeoutErrorはメッセージをそのまま使う', () => {
    expect(toAdminErrorMessage(new FetchTimeoutError()))
      .toBe('通信がタイムアウトしました。再試行してください。')
  })

  it('未知の例外は英語を出さず日本語の既定文言にする', () => {
    const message = toAdminErrorMessage(new TypeError('Failed to fetch'))

    expect(message).not.toContain('Failed to fetch')
    expect(message).toBe('通信エラーが発生しました。しばらくしてから再試行してください。')
  })

  it('Error以外が投げられても日本語の既定文言にする', () => {
    expect(toAdminErrorMessage('boom')).toBe('通信エラーが発生しました。しばらくしてから再試行してください。')
  })
})
